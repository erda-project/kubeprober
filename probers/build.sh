#!/bin/bash

set -x -o errexit -o nounset
cd "$(dirname "${BASH_SOURCE[0]}")"

function show_help {
    echo "build prober apps into one image"
    echo ""
    echo "Usage:"
    echo "    build --prober=xxx --apps=app1,app2,..."
    echo ""
    echo ""
    echo "Common Flags:"
    echo "    --prober                   specify the probe to build"
    echo "    --apps                     specify the apps under probe to build, all if not specified"
    echo ""
}

function flag_value {
    echo "$1" | sed 's/^[^=]*=//g'
}

if [[ $# -eq 0 ]]; then
    show_help
    exit 0
fi

PROBER=""
APPS=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        -h | --help)
            show_help
            exit 0
            ;;
        --prober*)
            PROBER=$(flag_value "$1")
            shift
            ;;
        --apps*)
            APPS=$(flag_value "$1")
            shift
            ;;
        *)
            echo "unknown flag: $1"
            show_help
            exit 1
            ;;
    esac
done

# check probe
if [ -z "$PROBER" ]; then
  echo "warning: empty probe"
  exit 1
fi

cd $PROBER
# check apps
if [ -z "$APPS" ]; then
  APPS=($(find * -maxdepth 0 -type d))
else
  APPS=($(echo $APPS | sed 's/,/ /g'))
fi

function check_file_type {
  # $1: app_name
  if [ -f "$1/main.go" ]; then
    echo "golang"
  elif [ -f "$1/main.sh" ]; then
    echo "shell"
  else
    echo ""
  fi
}

function buildcopy {
  # $1: app_name
  ft=$(check_file_type "$1")
  if [ "$ft" == "golang" ]; then
    # build golang app under the prober
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod readonly -a -o "$1"/main "$1"/main.go
    mkdir -pv /checkers/"$1"
    cp -rf "$1"/main /checkers/"$1"
  elif [ "$ft" == "shell" ]; then
    echo "shell type app $1, ignore build"
    mkdir -pv /checkers/"$1"
    cp -rf "$1" /checkers
    mv /checkers/$1/main.sh /checkers/$1/main
  else
    echo "no main file under path $1, or file type not support"
    exit 1
  fi
}

mkdir -pv /checkers
for app in ${APPS[@]}
do
  buildcopy "$app"
done





