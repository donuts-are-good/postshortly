#!/bin/bash

# Config variables
REPO_DIR="/root/postshortly"          # path to the Git repository
REMOTE_BRANCH="origin/master"        # remote branch to track
POLL_INTERVAL=60                     # interval in seconds between each poll
PROCESS_NAME="postshortly"            # name of the process to kill

# Declare latest_commit_message and last_processed_commit_hash as global variables
latest_commit_message=""
last_processed_commit_hash=""

# Check commit messages and perform actions
check_commit_message() {
    local commit_message="$1"
    local current_commit_hash="$2"

    # Check if the current commit hash is the same as the last processed one
    if [[ $current_commit_hash == $last_processed_commit_hash ]]; then
        echo -e "Commit hash $current_commit_hash has already been processed."
        return
    fi

    # Check if the commit message indicates a build and the commit hash is new
    if [[ $commit_message == "[BUILD]"* ]]; then
        echo -e "BUILD SPOTTED! Killing process ${PROCESS_NAME}"
        killall "${PROCESS_NAME}"
        # Update last processed commit hash
        last_processed_commit_hash=$current_commit_hash
    fi
}

# Main loop to pull and check the repository
while true; do
    # Navigate to the repository directory
    cd "${REPO_DIR}"

    # Fetch the latest changes from the remote repository
    git fetch --all

    # Check if there are any local changes that would prevent a pull
    if git status | grep -q 'Your branch is ahead'; then
        echo "Local changes detected, resetting to match remote..."
        git reset --hard "${REMOTE_BRANCH}"
    fi

    # Get the latest commit message and commit hash
    latest_commit_message=$(git log -1 --pretty=%B)
    current_commit_hash=$(git log -1 --pretty=%H)

    # Check the commit message and hash
    check_commit_message "${latest_commit_message}" "${current_commit_hash}"

    # Sleep for the specified interval before the next pull
    sleep "${POLL_INTERVAL}"
done
