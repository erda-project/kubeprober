// Copyright (c) 2021 Terminus, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ticket

import "time"

type ErrorResponse struct {
	Code string      `json:"code"`
	Msg  string      `json:"msg"`
	Ctx  interface{} `json:"ctx"`
}

type Header struct {
	Success bool          `json:"success"`
	Error   ErrorResponse `json:"err"`
}

type IssueType string

const (
	IssueTypeTicket IssueType = "TICKET"
)

type IssuePriority string

const (
	IssuePriorityUrgent IssuePriority = "URGENT"
	IssuePriorityHigh   IssuePriority = "HIGH"
	IssuePriorityNormal IssuePriority = "NORMAL"
	IssuePriorityLow    IssuePriority = "LOW"
)

type IssueStreamType string

const (
	ISTComment IssueStreamType = "Comment"
)

type Issue struct {
	ID       int64         `json:"id"`
	Title    string        `json:"title"`
	Priority IssuePriority `json:"priority"`
	State    int64         `json:"state"`
	Labels   []string      `json:"labels"`
}

type IssueCreateRequest struct {
	PlanStartedAt *time.Time    `json:"planStartedAt"`
	ProjectID     uint64        `json:"projectID"`
	IterationID   int64         `json:"iterationID"`
	Type          IssueType     `json:"type"`
	Title         string        `json:"title"`
	Content       string        `json:"content"`
	Priority      IssuePriority `json:"priority"`
	Assignee      string        `json:"assignee"`
	Labels        []string      `json:"labels"`
	UserID        string        `json:"userID"`
}

type IssueCreateResponse struct {
	Header
	Data uint64 `json:"data"`
}

type IssueUpdateRequest struct {
	Title    *string        `json:"title"`
	Content  *string        `json:"content"`
	State    *int64         `json:"state"`
	Priority *IssuePriority `json:"priority"`
	Assignee *string        `json:"assignee"`
	Labels   []string       `json:"labels"`
	ID       uint64         `json:"-"`
	UserID   string         `json:"userID"`
}

type IssueUpdateResponse struct {
	Header
	Data interface{} `json:"data"`
}

type IssuePagingRequest struct {
	ProjectID uint64  `json:"projectID"`
	Title     string  `json:"title"`
	State     []int64 `json:"state"`
	PageSize  uint64  `json:"pageSize"`
	OrderBy   string  `json:"orderBy"`
	Asc       bool    `json:"asc"`
}

type IssuePagingResponse struct {
	Header
	Data IssuePagingResponseData `json:"data"`
}

type IssuePagingResponseData struct {
	List []Issue `json:"list"`
}

type IssueStateRelation struct {
	StateID   int64  `json:"stateID"`
	StateName string `json:"stateName"`
}

type IssueStateRelationGetRequest struct {
	ProjectID uint64    `json:"projectID"`
	IssueType IssueType `json:"issueType"`
	UserID    string    `json:"userID"`
}

type IssueStateRelationGetResponse struct {
	Header
	Data []IssueStateRelation `json:"data"`
}

type OrgFetchResponse struct {
	Header
	Data OrgDTO `json:"data"`
}

type OrgDTO struct {
	ID uint64 `json:"id"`
}

type UserListResponse struct {
	Header
	Data UserListResponseData `json:"data"`
}

type UserListResponseData struct {
	Users []UserInfo `json:"users"`
}

type UserInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ProjectLabelListResponse struct {
	Header
	Data ProjectLabelListData `json:"data"`
}

type ProjectLabelListData struct {
	List []ProjectLabel `json:"list"`
}

type ProjectLabel struct {
	Name string `json:"name"`
}
