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

package options

import (
	"github.com/spf13/pflag"
)

type ReportStatusOptions struct {
	CheckerName string
	Status      string
	Message     string
}

// NewProbeMasterOptions creates a new NewProbeMasterOptions with a default config.
func NewReportStatusOptions() *ReportStatusOptions {
	o := &ReportStatusOptions{}

	return o
}

// AddFlags returns flags for a specific yurthub by section name
func (o *ReportStatusOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.CheckerName, "name", o.CheckerName, "The name of the checker.")
	fs.StringVar(&o.Status, "status", o.Status, "The status of the checker.")
	fs.StringVar(&o.Message, "message", o.Message, "The message of the checker .")
}
