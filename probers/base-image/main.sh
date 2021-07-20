#!/bin/bash

#execute checker in loop
for cmd in $(echo $PROBE_LIST | sed 's/,/ /g')
do
  chmod u+x /checker/$cmd
  /checker/$cmd
done