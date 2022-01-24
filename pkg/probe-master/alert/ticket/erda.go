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
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/klog"

	erda_api "github.com/erda-project/erda/apistructs"
	"github.com/erda-project/kubeprober/apistructs"
)

type ErdaIdentity struct {
	UserName   string
	Password   string
	OrgName    string
	ProjectId  uint64
	OpenapiUrl string
	client     *resty.Client

	UserID    string
	OrgID     uint64
	SessionID string

	Assignee string
	StateIds []int64
	StateId  int64
}

func Init(loginUser, loginPassword, openapiURL, orgName string, projectId uint64) error {
	erdaIdentity := &ErdaIdentity{
		UserName:   loginUser,
		Password:   loginPassword,
		OpenapiUrl: openapiURL,
		OrgName:    orgName,
		ProjectId:  projectId,
		Assignee:   "1001863", // erda-bot

		client: resty.New().SetRetryCount(3).SetRetryWaitTime(3 * time.Second).SetRetryMaxWaitTime(20 * time.Second),
	}

	err := erdaIdentity.GetUserOrgInfo()
	if err != nil {
		return err
	} else {
		sender = erdaIdentity

		err = sender.GetTicketStates()
		if err != nil {
			klog.Errorf("failed to fetch states from erda: %+v\n", err)
			return err
		}
		err = sender.GetAssignee()
		if err != nil {
			klog.Errorf("failed to fetch assignee for sre: %+v\n", err)
		}
	}

	return nil
}

func (u *ErdaIdentity) GetUserOrgInfo() error {
	var err error

	defer func() {
		if err != nil {
			logrus.Errorf("get user org info failed, identity info: %+v", err)
		}
	}()

	err = u.GetUserID()
	if err != nil {
		logrus.Errorf("get userID failed, error: %v", err)
		return err
	}

	err = u.GetOrgID()
	if err != nil {
		logrus.Errorf("get orgID failed, error: %v", err)
		return err
	}

	return nil
}

func (u *ErdaIdentity) GetUserID() error {
	resp, err := u.client.R().
		SetFormData(map[string]string{"username": u.UserName, "password": u.Password}).
		Post(strings.Join([]string{u.OpenapiUrl, "/login"}, ""))
	if err != nil {
		logrus.Errorf("login failed, error: %v", err)
		return err
	}

	i := apistructs.UserInfo{}
	err = unmarshalResponse(resp, &i)
	if err != nil {
		logrus.Errorf("unmarshal response failed, error: %v", err)
		return err
	}

	if i.SessionID == "" {
		err := fmt.Errorf("login failed, get empty sessionid")
		logrus.Errorf(err.Error())
		return err
	}

	u.UserID = i.UserID
	u.SessionID = i.SessionID

	logrus.Infof("login successfully!!! userInfo: %+v", i)

	return nil
}

func (u *ErdaIdentity) GetOrgID() error {
	resp, err := u.client.R().
		SetCookie(&http.Cookie{Name: "OPENAPISESSION", Value: u.SessionID}).
		SetHeader("USER-ID", u.UserID).
		Get(fmt.Sprintf("%s/api/orgs/%s", u.OpenapiUrl, u.OrgName))
	if err != nil {
		return err
	}

	r := erda_api.OrgFetchResponse{}
	err = unmarshalResponse(resp, &r)
	if err != nil {
		logrus.Errorf("unmarshal response failed, error: %v", err)
		return err
	}

	if !r.Success || r.Error.Msg != "" {
		err = fmt.Errorf("%v", r.Error)
		return err
	}

	if r.Data.ID < 1 {
		err = fmt.Errorf("invalid orgID: %v", r.Data.ID)
		return err
	}

	u.OrgID = r.Data.ID
	logrus.Infof("get org info successfully, orgInfo: %v", r.Data)

	return nil
}

func unmarshalResponse(r *resty.Response, o interface{}) error {
	if r == nil {
		err := fmt.Errorf("empty response")
		return err
	}

	if r.StatusCode() != 200 || r.Error() != nil {
		err := fmt.Errorf("status code: %v, body: %v error: %v", r.StatusCode(), string(r.Body()), r.Error())
		logrus.Errorf(err.Error())
		return err
	}

	if err := json.Unmarshal(r.Body(), o); err != nil {
		return fmt.Errorf("failed to Unmarshal JSON, err:%s，body :%s", err, string(r.Body()))
	}

	return nil
}

func (u *ErdaIdentity) GetTicketStates() error {
	req := &erda_api.IssueStateRelationGetRequest{}
	req.ProjectID = u.ProjectId
	req.IssueType = erda_api.IssueTypeTicket
	req.UserID = u.UserID
	rs, err := u.GetStateRelations(req)
	if err != nil {
		return err
	}

	for _, r := range rs {
		if r.StateName == "待处理" {
			u.StateIds = append(u.StateIds, r.StateID)
			u.StateId = r.StateID
		} else if r.StateName == "重新打开" {
			u.StateIds = append(u.StateIds, r.StateID)
		}
	}

	if len(u.StateIds) == 0 {
		logrus.Warnf("fetch states for project %d from erda failed ", u.ProjectId)
	}

	return nil
}

func (u *ErdaIdentity) GetAssignee() error {
	today := time.Now().Format("2006-01-02")
	resp, err := u.client.R().Get(fmt.Sprintf("https://onduty.app.terminus.io/sre?date=%s", today))
	if err != nil {
		return err
	}

	c := string(resp.Body())

	var username string
	lines := strings.Split(c, "\n")
	if strings.Contains(lines[0], "有事换班，今日值班请联系: ") {
		username = strings.TrimSpace(strings.Split(lines[0], "有事换班，今日值班请联系: ")[1])
	} else {
		username = strings.TrimSpace(strings.Split(lines[0], " ")[1])
	}
	user, err := u.SearchUser(username)
	if err != nil {
		return err
	}

	u.Assignee = user.ID

	return nil
}

func (u *ErdaIdentity) SearchUser(username string) (*erda_api.UserInfo, error) {
	resp, err := u.client.R().
		SetCookie(&http.Cookie{Name: "OPENAPISESSION", Value: u.SessionID}).
		SetHeader("USER-ID", u.UserID).
		SetHeader("Org-ID", strconv.FormatUint(u.OrgID, 10)).
		SetQueryParam("q", username).
		Get(fmt.Sprintf("%s/api/users/actions/search", u.OpenapiUrl))
	if err != nil {
		return nil, err
	}

	r := erda_api.UserListResponse{}
	err = unmarshalResponse(resp, &r)
	if err != nil {
		logrus.Errorf("unmarshal response failed, error: %v", err)
		return nil, err
	}
	if !r.Success || r.Error.Msg != "" {
		err = fmt.Errorf("%v", r.Error)
		return nil, err
	}

	l := len(r.Data.Users)
	if l <= 0 {
		return nil, errors.New(fmt.Sprintf("not found user %s, in erda", username))
	} else if l > 1 {
		logrus.Warnf("more than one user find with name %s, only choose one", username)
	}

	return &r.Data.Users[0], nil
}

func (u *ErdaIdentity) CreateIssue(req *erda_api.IssueCreateRequest) error {
	resp, err := u.client.R().SetBody(req).
		SetCookie(&http.Cookie{Name: "OPENAPISESSION", Value: u.SessionID}).
		SetHeader("USER-ID", u.UserID).
		SetHeader("Org-ID", strconv.FormatUint(u.OrgID, 10)).
		Post(strings.Join([]string{u.OpenapiUrl, "/api/issues"}, ""))
	if err != nil {
		return err
	}

	r := erda_api.IssueCreateResponse{}
	err = unmarshalResponse(resp, &r)
	if err != nil {
		logrus.Errorf("unmarshal response failed, error: %v", err)
		return err
	}
	if !r.Success || r.Error.Msg != "" {
		err = fmt.Errorf("%v", r.Error)
		return err
	}

	return nil
}

func (u *ErdaIdentity) UpdateIssue(req *erda_api.IssueUpdateRequest) error {
	resp, err := u.client.R().SetBody(req).
		SetCookie(&http.Cookie{Name: "OPENAPISESSION", Value: u.SessionID}).
		SetHeader("USER-ID", u.UserID).
		SetHeader("Org-ID", strconv.FormatUint(u.OrgID, 10)).
		Put(fmt.Sprintf("%s/api/issues/%d", u.OpenapiUrl, req.ID))
	if err != nil {
		return err
	}

	r := erda_api.IssueUpdateResponse{}
	err = unmarshalResponse(resp, &r)
	if err != nil {
		logrus.Errorf("unmarshal response failed for issue %d, error: %v", req.ID, err)
		return err
	}
	if !r.Success || r.Error.Msg != "" {
		err = fmt.Errorf("%v", r.Error)
		return err
	}

	return nil
}

func (u *ErdaIdentity) PagingIssue(req *erda_api.IssuePagingRequest) ([]erda_api.Issue, error) {
	resp, err := u.client.R().
		SetCookie(&http.Cookie{Name: "OPENAPISESSION", Value: u.SessionID}).
		SetHeader("USER-ID", u.UserID).
		SetHeader("Org-ID", strconv.FormatUint(u.OrgID, 10)).
		SetQueryParam("projectID", strconv.FormatUint(req.ProjectID, 10)).
		SetQueryParam("title", req.Title).
		SetQueryParam("orderBy", "planStartedAt").
		SetQueryParam("asc", "false").
		SetQueryParam("state", strconv.FormatInt(u.StateId, 10)).
		Get(strings.Join([]string{u.OpenapiUrl, "/api/issues"}, ""))

	if err != nil {
		return nil, err
	}

	r := erda_api.IssuePagingResponse{}
	err = unmarshalResponse(resp, &r)
	if err != nil {
		logrus.Errorf("unmarshal response failed, error: %v", err)
		return nil, err
	}
	if !r.Success || r.Error.Msg != "" {
		err = fmt.Errorf("%v", r.Error)
		return nil, err
	}

	return r.Data.List, nil
}

func (u *ErdaIdentity) GetStateRelations(req *erda_api.IssueStateRelationGetRequest) ([]erda_api.IssueStateRelation, error) {
	resp, err := u.client.R().SetBody(req).
		SetCookie(&http.Cookie{Name: "OPENAPISESSION", Value: u.SessionID}).
		SetHeader("USER-ID", u.UserID).
		SetHeader("Org-ID", strconv.FormatUint(u.OrgID, 10)).
		SetQueryParam("projectID", strconv.FormatUint(u.ProjectId, 10)).
		Get(strings.Join([]string{u.OpenapiUrl, "/api/issues/actions/get-state-relations"}, ""))
	if err != nil {
		return nil, err
	}

	r := &erda_api.IssueStateRelationGetResponse{}
	err = unmarshalResponse(resp, &r)
	if err != nil {
		logrus.Errorf("unmarshal response failed, error: %v", err)
		return nil, err
	}
	if !r.Success || r.Error.Msg != "" {
		err = fmt.Errorf("%v", r.Error)
		return nil, err
	}

	return r.Data, nil
}
