package ticket

import erda_api "github.com/erda-project/erda/apistructs"

type IssuePriority = erda_api.IssuePriority
type IssueType = erda_api.IssueType

const (
	IssueTypeTicket     = erda_api.IssueTypeTicket
	IssuePriorityUrgent = erda_api.IssuePriorityUrgent
	IssuePriorityHigh   = erda_api.IssuePriorityHigh
	IssuePriorityNormal = erda_api.IssuePriorityNormal
	IssuePriorityLow    = erda_api.IssuePriorityLow
)
