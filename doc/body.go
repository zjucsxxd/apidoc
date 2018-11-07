// Copyright 2018 by caixw, All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package doc

import (
	"bytes"
	"strconv"
	"strings"

	"github.com/caixw/apidoc/internal/locale"
)

// Request 表示用户请求所表示的数据。
type Request = Body

// Response 表示一次请求或是返回的数据。
type Response struct {
	Body
	Status int `yaml:"status" json:"status"`
}

// API 和 Doc 都有这个属性，且都需要 parseResponse 方法。
// 抽象为一个嵌套对象使用。
type responses struct {
	Responses []*Response `yaml:"responses,omitempty" json:"responses,omitempty"`
}

// API 和 Callback 共同需要的属性
type requests struct {
	Requests []*Request `yaml:"requests,omitempty" json:"requests,omitempty"`
}

// Body 表示请求和返回的共有内容
type Body struct {
	Mimetype string     `yaml:"mimetype,omitempty" json:"mimetype,omitempty"`
	Headers  []*Header  `yaml:"headers,omitempty" json:"headers,omitempty"`
	Type     *Schema    `yaml:"type" json:"type"`
	Examples []*Example `yaml:"examples,omitempty" json:"examples,omitempty"`
}

// Header 报头
type Header struct {
	Name     string `yaml:"name" json:"name"`                             // 参数名称
	Summary  string `yaml:"summary" json:"summary"`                       // 参数介绍
	Optional bool   `yaml:"optional,omitempty" json:"optional,omitempty"` // 是否可以为空
}

// Example 示例
type Example struct {
	Mimetype string `yaml:"mimetype" json:"mimetype"`
	Summary  string `yaml:"summary,omitempty" json:"summary,omitempty"`
	Value    string `yaml:"value" json:"value"` // 示例内容
}

// 解析示例代码，格式如下：
//  @apiExample application/json
//  {
//      "id": 1,
//      "name": "name",
//  }
func (body *Body) parseExample(tag *lexerTag) {
	lines := tag.lines(2)
	if len(lines) != 2 {
		tag.err(locale.ErrInvalidFormat)
		return
	}

	words := splitWords(lines[0], 2)

	if body.Examples == nil {
		body.Examples = make([]*Example, 0, 3)
	}

	example := &Example{
		Mimetype: string(words[0]),
		Value:    string(lines[1]),
	}
	if len(words) == 2 { // 如果存在简介
		example.Summary = string(words[1])
	}

	body.Examples = append(body.Examples, example)
}

var requiredBytes = []byte("required")

func isOptional(data []byte) bool {
	return !bytes.Equal(bytes.ToLower(data), requiredBytes)
}

// 解析 @apiHeader 标签，格式如下：
//  @apiheader content-type required desc
func (body *Body) parseHeader(tag *lexerTag) {
	data := tag.words(3)
	if len(data) != 3 {
		tag.err(locale.ErrInvalidFormat)
		return
	}

	if body.Headers == nil {
		body.Headers = make([]*Header, 0, 3)
	}

	body.Headers = append(body.Headers, &Header{
		Name:     string(data[0]),
		Summary:  string(data[2]),
		Optional: isOptional(data[1]),
	})
}

// 解析 @apiparam 标签，格式如下：
//  @apiparam group object reqiured desc
func (body *Body) parseParam(tag *lexerTag) {
	data := tag.words(4)
	if len(data) != 4 {
		tag.err(locale.ErrInvalidFormat)
		return
	}

	if err := body.Type.build(data[0], data[1], data[2], data[3]); err != nil {
		tag.errWithError(err, locale.ErrInvalidFormat)
		return
	}
}

func (resps *responses) parseResponse(l *lexer, tag *lexerTag) {
	if resps.Responses == nil {
		resps.Responses = make([]*Response, 0, 3)
	}

	resp, ok := newResponse(l, tag)
	if !ok {
		return
	}
	resps.Responses = append(resps.Responses, resp)
}

// 解析 @apiRequest 及其子标签，格式如下：
//  @apirequest object * 通用的请求主体
//  @apiheader name optional desc
//  @apiheader name optional desc
//  @apiparam count int optional desc
//  @apiparam list array.string optional desc
//  @apiparam list.id int optional desc
//  @apiparam list.name int reqiured desc
//  @apiparam list.groups array.string optional.xxxx desc markdown enum:
//   * xx: xxxxx
//   * xx: xxxxx
//  @apiexample application/json summary
//  {
//   count: 5,
//   list: [
//     {id:1, name: 'name1', 'groups': [1,2]},
//     {id:2, name: 'name2', 'groups': [1,2]}
//   ]
//  }
func (reqs *requests) parseRequest(l *lexer, tag *lexerTag) {
	data := tag.words(3)
	if len(data) < 2 {
		tag.err(locale.ErrInvalidFormat)
		return
	}

	if reqs.Requests == nil {
		reqs.Requests = make([]*Request, 0, 3)
	}

	var desc []byte
	if len(data) == 3 {
		desc = data[2]
	}

	req := &Request{
		Mimetype: string(data[1]),
		Type:     &Schema{},
	}
	reqs.Requests = append(reqs.Requests, req)

	if err := req.Type.build(nil, data[0], nil, desc); err != nil {
		tag.errWithError(err, locale.ErrInvalidFormat)
		return
	}

LOOP:
	for tag := l.tag(); tag != nil; tag = l.tag() {
		fn := req.parseExample
		switch strings.ToLower(tag.Name) {
		case "@apiexample":
			fn = req.parseExample
		case "@apiheader":
			fn = req.parseHeader
		case "@apiparam":
			fn = req.parseParam
		default:
			l.backup(tag)
			break LOOP
		}

		fn(tag)
	}
}

// 解析 @apiResponse 及子标签，格式如下：
//  @apiresponse 200 array.object * 通用的返回内容定义
//  @apiheader content-type required desc
//  @apiparam id int reqiured desc
//  @apiparam name string reqiured desc
//  @apiparam group object reqiured desc
//  @apiparam group.id int reqiured desc
func newResponse(l *lexer, tag *lexerTag) (resp *Response, ok bool) {
	data := tag.words(4)
	if len(data) < 3 {
		tag.err(locale.ErrInvalidFormat)
		return nil, false
	}

	status, err := strconv.Atoi(string(data[0]))
	if err != nil {
		tag.err(locale.ErrInvalidFormat)
		return nil, false
	}

	var desc []byte
	if len(data) == 4 {
		desc = data[3]
	}

	s := &Schema{}
	if err := s.build(nil, data[1], nil, desc); err != nil {
		tag.errWithError(err, locale.ErrInvalidFormat)
		return nil, false
	}
	resp = &Response{
		Status: status,
		Body: Body{
			Mimetype: string(data[2]),
			Type:     s,
		},
	}

LOOP:
	for tag := l.tag(); tag != nil; tag = l.tag() {
		fn := resp.parseExample
		switch strings.ToLower(tag.Name) {
		case "@apiexample":
			fn = resp.parseExample
		case "@apiheader":
			fn = resp.parseHeader
		case "@apiparam":
			fn = resp.parseParam
		default:
			l.backup(tag)
			break LOOP
		}

		fn(tag)
	}

	return resp, true
}
