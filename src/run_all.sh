#!/usr/bin/env bash
#
# Purpose: Run all compiled binaries except one(s) which are being debugged under an IDE (optional)
#          Send logs to ./logs/NAME.log where NAME is the binary name. Note these are not log rotated.
#          Any arguments are passed to all binaries, e.g: ./run_all.sh local
#          Upon receipt of CTRL-C, kill all tracked binary PIDs
#          This does not restart binaries which have terminated
#

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

src_paths_to_build=(
  "csms-server"
  "device-manager"
  "message-manager"
  "session"
)

logPath="$SCRIPT_DIR/logs"
[ ! -e "$logPath" ] && \mkdir "$logPath"
pidPath="$SCRIPT_DIR/pids"
[ ! -e "$pidPath" ] && \mkdir "$pidPath"

isWindows=0
[[ "$(uname)" == "MINGW"* ]] && isWindows=1

echo "INFO: logPath: $logPath"
echo "INFO: pidPath: $pidPath"
echo
echo "INFO: This will start the following binaries: ${src_paths_to_build[@]}"
echo "INFO: It will monitor their PIDs and capture logging to a file. It does *not* restart binaries that have terminated"
echo
echo "Q: Optionally enter binaries to exclude from starting, separated by a space: e.g client"
echo
if [ "$ans" == "" ]; then
  read ANS
else  
  echo ans=$ans
  ANS="$ans"
fi
  
binaryList="$(printf " %s" "${src_paths_to_build[@]}")" 
binaryList="${binaryList:1}"

for path in "${src_paths_to_build[@]}"; do
  [[ "$ANS" == *"$path"* ]] && continue # matches excluded binary
  
  binPath="$SCRIPT_DIR/$path"
	if [ -e "$binPath" ]; then
    logFile="$logPath/$path.log"
    pidFile="$pidPath/$path.pid"

    cd "$binPath"
    
    if [ $isWindows == 1 ]; then
       fileBinPath="${path}.exe"
    else
       fileBinPath="$path"
    fi

    ./"$fileBinPath" $* &>> "$logFile" &
    PID=$!
    echo $PID > "$pidFile"
    echo "INFO: $fileBinPath started with PID: $PID"
    cd "$SCRIPT_DIR"
  else
    echo "ERROR: $binPath does not exist"
  fi
done

exitHandler() {
  rc=$1
  [ "$1" == "" ] && rc=0
  echo "INFO: Script caught signal: $rc"
  for file in "$pidPath"/*; do
     pid="$(cat "$file")"
     if [ "$pid" != "" ]; then
       process="$(echo $file | grep -o '[^\/]*$' | awk -F'.' {'print $1'})"
       echo "INFO: Send SIGTERM to: $pid ($process)"
       \kill $pid &>/dev/null
     fi
     
     [ -e "$file" ] && \rm "$file"
  done
  \exit $rc
}

trap "exitHandler" TERM USR2 INT
( (
  sleep 2
  while [ 1 == 1 ]
  do
     clear
     pidList="$(echo "$binaryList" | sed 's/ /\\\|/g')"
     echo "INFO: PIDS..."
     echo
     \ps -ef | \grep "$pidList"
     echo
     echo "INFO: CTRL-C to quit..."
     sleep 1
     clear
  done
) 2>&1 )
