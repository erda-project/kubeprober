#!/bin/bash

function checker2_item1_check() {
  # checker2 item1
	# do real check ..., and report check status
	report-status --name=checker2_item1 --status=pass --message="-"
}

function checker2_item2_check() {
  # checker2 item2
	# do real check ..., and report check status
	report-status --name=checker2_item2 --status=error --message="checker2 item2 failed, reason: ..."
}

checker2_item1_check
checker2_item2_check