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
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/klog"

	erda_api "github.com/erda-project/erda/apistructs"
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

func sendIssue(t *Ticket) error {
	issue, err := existIssue(t)
	if err != nil {
		return err
	}

	if issue != nil {
		sameContent := strings.Contains(issue.Content, t.Content)
		newLabel, existLabel := existLabels(issue.Labels, t.newLabels())

		if sameContent && existLabel {
			return nil
		}

		reqU := &erda_api.IssueUpdateRequest{}
		reqU.ID = uint64(issue.ID)
		reqU.Title = &issue.Title
		reqU.Priority = &issue.Priority

		if t.Kind == PassTicket {
			// duplicate msg
			if issue.State == sender.NoprocessStateId {
				return nil
			}

			reqU.State = &sender.NoprocessStateId
		} else {
			reqU.State = &issue.State
		}

		reqU.Assignee = &sender.Assignee
		reqU.Content = &t.Content

		reqU.UserID = sender.UserID

		reqU.Labels = newLabel

		return updateIssue(reqU)
	}

	// no need to send issue
	if t.Kind == PassTicket {
		return nil
	}

	return createIssue(t)
}

func existLabels(oldL, newL []string) ([]string, bool) {
	newLabels := []string{}
	oldMap := map[string]interface{}{}
	for _, l := range oldL {
		oldMap[l] = struct{}{}
		newLabels = append(newLabels, l)
	}

	exist := true
	for _, nl := range newL {
		if _, ok := oldMap[nl]; !ok {
			exist = false
			newLabels = append(newLabels, nl)
		}
	}

	return newLabels, exist
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
