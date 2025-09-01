#!/usr/bin/env bash
# Purpose: Package up the CSMS binaries

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

[ "$SYSTEM_DEBUG" == "1" ] && set -x

while [[ $# -gt 0 ]]; do
  case "$1" in
  --output)
    shift
    output_dir="$1"
    shift
    ;;
  --version)
    shift
    version="$1"
    shift
    ;;
  *)
    echo "ERROR: Unexpected parameter $1"
    exit 1
    ;;
  esac
done

while [ "$version" == "" ]; do
  echo "Please enter package version number to use: "
  read version
done

name="ocpp_csms"
ver="${version}"
output_dir=${output_dir:-"$SCRIPT_DIR"}
archive_parent_path="${name}-${ver}"
pkg_output_dir="$SCRIPT_DIR/output/$archive_parent_path"
archive="${output_dir}/${name}-${ver}.tgz"
db_dir="$pkg_output_dir/db"

log() {
  DT=$(date +"%Y%m%d %H:%M:%S")
  \echo "$DT : $*"
}

check_rc() {
  [ "$2" != 0 ] && \echo "ERROR: in \"$1\" command: $2" && \exit "$2"
}

[ ! -d "$pkg_output" ] && \mkdir -p "$pkg_output_dir"

bins_to_include=(
  csms-server
  device-manager
  message-manager
  session
)

if ! command -v "dos2unix" &>/dev/null; then
  log "ERROR: Missing binary dos2unix"
  exit 1
fi
    
ext=".exe"
if [ "$(uname)" == "Linux" ]; then
  ext=""
fi

for path in "${bins_to_include[@]}"; do
  dest_path="$pkg_output_dir/${path}/"
  [ ! -d "$dest_path" ] && \mkdir "$dest_path"
  
  \cp "$SCRIPT_DIR/$path/${path}${ext}" "${dest_path}"
  check_rc "cp $path" "$?"
done

cfg_path="${pkg_output_dir}/cfg/"
[ ! -d "$cfg_path" ] && \mkdir "$cfg_path"
\cp "$SCRIPT_DIR/cfg/conf.example.yaml" "${cfg_path}/conf.yaml"
check_rc "cp cfg" "$?"

\cp "$SCRIPT_DIR"/run_all.sh "$pkg_output_dir"
\cp "$SCRIPT_DIR"/../README.md "$pkg_output_dir"
\cp "$SCRIPT_DIR"/device-manager/deviceManager.http "$pkg_output_dir/device-manager/"

[ ! -d "$db_dir" ] && \mkdir "$db_dir"

\dos2unix "$pkg_output_dir/*.sh"
\dos2unix "$cfg_path/*.yaml"

cwd="$(pwd)"
\cd "$pkg_output_dir/../"
\tar -zcvf "${archive}" "$archive_parent_path"
#check_rc "tar" "$?"
\cd "$cwd"
ls -la "${archive}"