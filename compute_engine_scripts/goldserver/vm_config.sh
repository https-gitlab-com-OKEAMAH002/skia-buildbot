#!/bin/bash
#
# This file contains constants for the shell scripts which interact
# with the skia-gold Google Compute Engine instance.
#
# Copyright 2014 Google Inc. All Rights Reserved.

set -e

# Sets all constants in compute_engine_cfg.py as env variables.
$(python ../compute_engine_cfg.py)
if [ $? != "0" ]; then
  echo "Failed to read compute_engine_cfg.py!"
  exit 1
fi

# Shared scope that is inherited from compute_engine_cfg.py.
GOLD_SCOPES="$SCOPES"
GOLD_SOURCE_IMAGE="skia-systemd-pushable-base"

case "$VM_ID" in
  prod)
    GOLD_MACHINE_TYPE=n1-highmem-32
    GOLD_IP_ADDRESS=104.154.112.104
    GOLD_DATA_DISK_SIZE="2TB"
    ;;

  pdfium)
    GOLD_MACHINE_TYPE=n1-highmem-16
    GOLD_IP_ADDRESS=104.154.112.106
    GOLD_DATA_DISK_SIZE="500GB"
    ;;

  # For testing only. Destroy after creation.
  testinstance)
    GOLD_MACHINE_TYPE=n1-highmem-16
    GOLD_IP_ADDRESS=104.154.112.111
    GOLD_DATA_DISK_SIZE="500GB"
    GOLD_SCOPES="$GOLD_SCOPES,https://www.googleapis.com/auth/androidbuild.internal"
    ;;

  *)
    # There must be a target instance id provided.
    echo "Usage: $0 {prod | pdfium | testinstance}"
    echo "   An instance id must be provided as the first argument."
    exit 1
    ;;

esac

# The base names of the VM instances. Actual names are VM_NAME_BASE-name-zone
VM_NAME_BASE=${VM_NAME_BASE:="skia"}

# The name of instance where gold is running on.
INSTANCE_NAME=${VM_NAME_BASE}-gold-$VM_ID

# The name of the data disk
GOLD_DATA_DISK_NAME=${INSTANCE_NAME}-data