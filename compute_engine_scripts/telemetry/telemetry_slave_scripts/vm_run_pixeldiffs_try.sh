#!/bin/bash
#
# Downloads the nopatch and withpatch chromium builds and calls the
# capture_and_compare_pixeldiffs.py script to download webpages and create the
# output HTML.
#
# The script should be run from the skia-telemetry-slave GCE instance's
# /home/default/skia-repo/buildbot/compute_engine_scripts/telemetry/telemetry_slave_scripts
# directory.
#
# Copyright 2014 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)


function usage() {
  cat << EOF

usage: $0 options

This script runs render pictures on SKPs with the specified patch and then runs
render pictures on SKPs without the patch. The two sets of images are then
compared and a JSON file is outputted detailing all failures.

OPTIONS:
  -h Show this message
  -n The slave_num of this cluster telemetry slave
  -b The name of the no patch Chromium build dir in Google Storage
  -p The name of the with patch Chromium build dir in Google Storage
  -s The start rank this slave should process from the alexa CSV
  -e The end rank this slave should process from the alexa CSV
  -r The runid (typically requester + timestamp)
  -g The Google Storage location where the log file should be uploaded to
  -o The Google Storage location where the output files should be uploaded to
  -l The location of the log file
EOF
}

while getopts "hn:b:p:s:e:r:g:o:l:" OPTION
do
  case $OPTION in
    h)
      usage
      exit 1
      ;;
    n)
      SLAVE_NUM=$OPTARG
      ;;
    b)
      CHROMIUM_BUILD_DIR_NO_PATCH=$OPTARG
      ;;
    p)
      CHROMIUM_BUILD_DIR_WITH_PATCH=$OPTARG
      ;;
    s)
      START_RANK=$OPTARG
      ;;
    e)
      END_RANK=$OPTARG
      ;;
    r)
      RUN_ID=$OPTARG
      ;;
    g)
      LOG_FILE_GS_LOCATION=$OPTARG
      ;;
    o)
      OUTPUT_FILE_GS_LOCATION=$OPTARG
      ;;
    l)
      LOG_FILE=$OPTARG
      ;;
    ?)
      usage
      exit
      ;;
  esac
done

if [[ -z $SLAVE_NUM ]] || [[ -z $CHROMIUM_BUILD_DIR_NO_PATCH ]] || \
   [[ -z $CHROMIUM_BUILD_DIR_WITH_PATCH ]] || [[ -z $RUN_ID ]] || \
   [[ -z $LOG_FILE ]] || [[ -z $LOG_FILE_GS_LOCATION  ]] || \
   [[ -z $OUTPUT_FILE_GS_LOCATION ]] || [[ -z $START_RANK ]] || \
   [[ -z $END_RANK ]]
then
  usage
  exit 1
fi

source vm_utils.sh

WORKER_FILE=PIXELDIFFS.$RUN_ID
create_worker_file $WORKER_FILE

if [ -e /etc/boto.cfg ]; then
  # Move boto.cfg since it may interfere with the ~/.boto file.
  sudo mv /etc/boto.cfg /etc/boto.cfg.bak
fi

# Download the nopatch and withpatch builds from Google Storage.
build_dir_array=( "${CHROMIUM_BUILD_DIR_NO_PATCH}" "${CHROMIUM_BUILD_DIR_WITH_PATCH}" )
for build_dir in "${build_dir_array[@]}"
do
  rm -rf /home/default/storage/chromium-builds/${build_dir}*;
  mkdir -p /home/default/storage/chromium-builds/${build_dir};
  gsutil cp -R gs://chromium-skia-gm/telemetry/chromium-builds/${build_dir}/* \
      /home/default/storage/chromium-builds/${build_dir}
  sudo chmod 777 /home/default/storage/chromium-builds/${build_dir}/content_shell
  sudo chmod 777 /home/default/storage/chromium-builds/${build_dir}/image_diff
done

# Download Alexa top 1M CSV from Google Storage.
gsutil cp gs://chromium-skia-gm/telemetry/pixeldiffs/csv/top-1m-${RUN_ID}.csv /tmp/top-1m.csv

# Start an Xvfb display on :0.
sudo Xvfb :0 -screen 0 1280x1024x24 &

OUTPUT_DIR=/home/default/storage/pixeldiffs/${RUN_ID}
mkdir -p $OUTPUT_DIR

# Run the "before" command pointing to the nopatch build.
DISPLAY=:0 python capture_and_compare_pixeldiffs.py \
    --additional_flags="--disable-setuid-sandbox --enable-software-compositing" \
    --output_dir=$OUTPUT_DIR --csv_path=/tmp/top-1m.csv \
    --chromium_out_dir=/home/default/storage/chromium-builds/${CHROMIUM_BUILD_DIR_NO_PATCH} \
    --gs_url_prefix=https://storage.cloud.google.com/chromium-skia-gm/telemetry/pixeldiffs/outputs/$RUN_ID/slave$SLAVE_NUM \
    --start_number=$START_RANK --end_number=$END_RANK --action=before

# Run the "after" command pointing to the withpatch build.
DISPLAY=:0 python capture_and_compare_pixeldiffs.py \
    --additional_flags="--disable-setuid-sandbox --enable-software-compositing" \
    --output_dir=$OUTPUT_DIR --csv_path=/tmp/top-1m.csv \
    --chromium_out_dir=/home/default/storage/chromium-builds/${CHROMIUM_BUILD_DIR_WITH_PATCH} \
    --gs_url_prefix=https://storage.cloud.google.com/chromium-skia-gm/telemetry/pixeldiffs/outputs/$RUN_ID/slave$SLAVE_NUM \
    --start_number=$START_RANK --end_number=$END_RANK --action=after

# Run the "compare" command.
DISPLAY=:0 python capture_and_compare_pixeldiffs.py \
    --additional_flags="--disable-setuid-sandbox --enable-software-compositing" \
    --output_dir=$OUTPUT_DIR --csv_path=/tmp/top-1m.csv \
    --chromium_out_dir=/home/default/storage/chromium-builds/${CHROMIUM_BUILD_DIR_WITH_PATCH} \
    --gs_url_prefix=https://storage.cloud.google.com/chromium-skia-gm/telemetry/pixeldiffs/outputs/$RUN_ID/slave$SLAVE_NUM \
    --start_number=$START_RANK --end_number=$END_RANK --action=compare


# Copy over artifacts generated by the capture_and_compare_pixeldiffs.py script
# to Google Storage.
for diff_file in $OUTPUT_DIR/real_world_impact/diff/*.png; do
  diff_file_basename=`basename $diff_file`
  # Copy the diff_file to Google Storage.
  gsutil cp -R $OUTPUT_DIR/real_world_impact/diff/$diff_file_basename ${OUTPUT_FILE_GS_LOCATION}/slave$SLAVE_NUM/diff/
  # Copy the corresponding before and actual to Google Storage.
  gsutil cp -R $OUTPUT_DIR/real_world_impact/before/$diff_file_basename ${OUTPUT_FILE_GS_LOCATION}/slave$SLAVE_NUM/before/
  gsutil cp -R $OUTPUT_DIR/real_world_impact/after/$diff_file_basename ${OUTPUT_FILE_GS_LOCATION}/slave$SLAVE_NUM/after/
done

# Set google.com ACLs on all diff/actual/before images.
artifacts_to_copy=( diff before after )
for artifact in "${artifacts_to_copy[@]}"
do
  gsutil acl ch -g google.com:READ ${OUTPUT_FILE_GS_LOCATION}/slave$SLAVE_NUM/$artifact/*
done

# Copy only diff.html and bad_urls.txt over with public-read ACL.
gsutil cp -a public-read $OUTPUT_DIR/real_world_impact/diff.html ${OUTPUT_FILE_GS_LOCATION}/slave$SLAVE_NUM/diff.html
gsutil cp -a public-read $OUTPUT_DIR/real_world_impact/data/bad_urls.txt ${OUTPUT_FILE_GS_LOCATION}/slave$SLAVE_NUM/bad_urls.txt

# Copy log file to Google Storage.
gsutil cp -a public-read $LOG_FILE ${LOG_FILE_GS_LOCATION}/slave${SLAVE_NUM}/

# Clean up.
rm -rf $OUTPUT_DIR
rm -rf $LOG_FILE
delete_worker_file $WORKER_FILE
