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
// See the License for the specific language governing permissions and
// limitations under the License.

package apistructs

type UserInfo struct {
	SessionID string `json:"sessionid"`
	UserName  string `json:"username"`
	UserID    string `json:"id"`
}

type ErdaConfig struct {
	TicketEnable bool
	OpenapiURL   string
	Username     string
	Password     string
	Org          string
	ProjectId    uint64
}

// TODO replicate because could not import erda v2+
type CommentIssueStreamBatchCreateRequest struct {
	IssueStreams []*CommentIssueStreamCreateRequest `json:"issueStreams,omitempty"`
}

type CommentIssueStreamCreateRequest struct {
	IssueID int64          `json:"issueID,omitempty"`
	Type    string         `json:"type,omitempty"`
	Content string         `json:"content,omitempty"`
	MrInfo  *MRCommentInfo `json:"mrInfo,omitempty"`
	UserID  string         `json:"userID,omitempty"`
}
type MRCommentInfo struct {
	AppID   int64  `json:"appID,omitempty"`
	MrID    int64  `json:"mrID,omitempty"`
	MrTitle string `json:"mrTitle,omitempty"`
}
