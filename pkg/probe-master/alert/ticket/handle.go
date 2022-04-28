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

package ticket

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/klog"

	erda_api "github.com/erda-project/erda/apistructs"
	"github.com/erda-project/kubeprober/apistructs"
)

var (
	sendIssueCh = make(chan *Ticket, 100)
	sender      *ErdaIdentity
)

func init() {
	initWorker()
}

func initWorker() {
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		for {
			select {
			case t := <-sendIssueCh:
				t.Title = fmt.Sprintf("%s-{%s}", t.Title, GetWeek())
				err := sendIssue(t)
				if err != nil {
					klog.Errorf("send ticket failed, %v", err)
				}
			case <-ticker.C:
				if sender != nil {
					err := sender.GetUserID()
					if err != nil {
						klog.Errorf("user login failed, %v", err)
					}

					err = sender.GetTicketStates()
					if err != nil {
						klog.Errorf("get ticket states failed, %v", err)
					}

					err = sender.GetAssignee()
					if err != nil {
						klog.Errorf("get assingee failed, %v", err)
					}

					err = sender.GetLabels()
					if err != nil {
						klog.Errorf("get labels failed, %v", err)
					}
				}
			}
		}
	}()
}

type TicketKind string

const (
	ErrorTicket TicketKind = "Error"
	PassTicket  TicketKind = "Pass"
)

type Ticket struct {
	Kind   TicketKind
	Labels []string

	Title    string
	Content  string
	Priority erda_api.IssuePriority
	Type     erda_api.IssueType
}

func GetWeek() string {
	timeLayout := "20060102"
	now := time.Now().Unix()
	datetime := time.Unix(now, 0).Format(timeLayout)

	loc, _ := time.LoadLocation("Asia/Shanghai")
	tmp, _ := time.ParseInLocation(timeLayout, datetime, loc)
	y, w := tmp.ISOWeek()
	return fmt.Sprintf("%d-%d", y, w)
}

func sendIssue(t *Ticket) error {
	issue, err := existIssue(t)
	if err != nil {
		return err
	}

	if issue != nil {

		reqU := &erda_api.IssueUpdateRequest{}
		reqU.ID = uint64(issue.ID)
		reqU.Title = &issue.Title
		reqU.Priority = &issue.Priority

		if t.Kind == PassTicket {
			// duplicate msg
			if issue.State != sender.TodoStateId &&
				issue.State != sender.ReopenStateId {
				return nil
			}
			reqU.State = &sender.NoprocessStateId
		} else {
			reqU.State = &issue.State
			if issue.State == sender.NoprocessStateId ||
				issue.State == sender.SolvedStateId {
				reqU.State = &sender.ReopenStateId
			}

			foundNoHandleLabel := false
			for _, l := range issue.Labels {
				if l == "暂不修复" {
					foundNoHandleLabel = true
				}
			}
			if foundNoHandleLabel && issue.State == sender.NoprocessStateId {
				return nil
			}
		}

		err = createIssueComment(issue.ID, t.Content)
		if err != nil {
			return err
		}

		reqU.Content = &t.Content
		reqU.Assignee = &sender.Assignee
		reqU.UserID = sender.UserID

		reqU.Labels = getLabels(issue.Labels, t.newLabels())

		return updateIssue(reqU)
	}

	// no need to send issue
	if t.Kind == PassTicket {
		return nil
	}

	return createIssue(t)
}

func createIssueComment(issueID int64, content string) error {
	comment := &apistructs.CommentIssueStreamCreateRequest{
		IssueID: issueID,
		Type:    string(erda_api.ISTComment),
		UserID:  sender.UserID,
		Content: content,
	}
	relateReq := apistructs.CommentIssueStreamBatchCreateRequest{
		IssueStreams: []*apistructs.CommentIssueStreamCreateRequest{comment},
	}
	return sender.CreateIssueComment(&relateReq)
}

func getLabels(oldL, newL []string) []string {
	newLabels := []string{}
	oldMap := map[string]interface{}{}
	for _, l := range oldL {
		oldMap[l] = struct{}{}
		newLabels = append(newLabels, l)
	}

	for _, nl := range newL {
		if _, ok := oldMap[nl]; !ok {
			newLabels = append(newLabels, nl)
		}
	}

	return newLabels
}

func updateIssue(req *erda_api.IssueUpdateRequest) error {
	return sender.UpdateIssue(req)
}

func createIssue(t *Ticket) error {
	now := time.Now()

	req := &erda_api.IssueCreateRequest{}
	req.Title = t.Title
	req.Content = t.Content
	req.Priority = t.Priority
	req.Type = t.Type
	req.PlanStartedAt = &now
	req.Assignee = sender.Assignee
	req.IterationID = -1

	req.UserID = sender.UserID
	req.ProjectID = sender.ProjectId

	logrus.Infof("server label %+v", sender.Labels)
	logrus.Infof("ticket label %+v", t.Labels)

	req.Labels = t.newLabels()

	return sender.CreateIssue(req)
}

func (t *Ticket) newLabels() []string {
	var labels []string
	for _, l := range t.Labels {
		if _, ok := sender.Labels[l]; ok {
			labels = append(labels, l)
		}
	}

	return labels
}

func existIssue(t *Ticket) (*erda_api.Issue, error) {
	req := &erda_api.IssuePagingRequest{}
	req.ProjectID = sender.ProjectId
	req.OrderBy = "plan_started_at"
	req.Asc = false
	req.Title = t.Title
	req.State = sender.StateIds
	req.PageSize = 1

	issues, err := sender.PagingIssue(req)
	if err != nil {
		return nil, err
	}

	var issue erda_api.Issue
	l := len(issues)
	if l == 0 {
		return nil, nil
	} else if l > 1 {
		logrus.Warn("more than one issue exist with same title")
	}
	issue = issues[0]

	return &issue, nil
}

func SendTicket(t *Ticket) {
	if sender != nil {
		sendIssueCh <- t
	}
}
