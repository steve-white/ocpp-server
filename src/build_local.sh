#!/usr/bin/env bash
#
# Purpose: Build the go binaries locally. Usage: ./build_local.sh
#          Note this should not be used for CI.
#

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

src_paths_to_build=(
  "csms-server/"
  "device-manager/"
  "message-manager/"
  "session/"
)

GOBIN=$GOPATH/bin
PATH=$PATH:$GOROOT/bin:$GOBIN

if [ "$1" == "clean" ]; then
  for path in $src_paths_to_build
  do
    echo "INFO: Cleaning ${path}..."
    \rm -f "${path:?}"/*
  done
  exit 0
fi

export BuildNumber="0.0.${EPOCHSECONDS}"
export BuildSourceVersion="$(git rev-parse --abbrev-ref HEAD)"
export currentDate="$(echo `date` | sed 's/ /_/g')"

echo "INFO: Version=$BuildNumber"
echo "INFO: CommitHash=$BuildSourceVersion"
echo "INFO: BuildTimestamp=$currentDate"
echo

# Function to process the relative path and filenames
build_bin() {

  localPath=$1
  cd "${SCRIPT_DIR}/$localPath"
  echo "INFO: Current build path:" $(pwd)
  
  export CGO_ENABLED=1
  configNamespace="sw/ocpp/csms/internal/service"
  go build --ldflags="-X ${configNamespace}.Version=$BuildNumber -X ${configNamespace}.CommitHash=$BuildSourceVersion -X ${configNamespace}.BuildTimestamp=$currentDate" .
  		
  [ "$?" != 0 ] && echo "ERROR: Problem building path[$?]: $localPath" && read ANS
}

for path in "${src_paths_to_build[@]}"; do
	build_bin $path
done

echo "INFO: Press enter to continue..."
read ANS
