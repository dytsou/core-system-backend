#!/bin/bash
# Run all k6 concurrency tests
# Usage: ./run-all.sh [config-file]

set -e

# Change to script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Load environment variables
if [ -n "$1" ]; then
    echo "Loading configuration from $1"
    set -a
    source "$1"
    set +a
else
    if [ -f .env ]; then
        set -a
        source .env
        set +a
    fi
fi

BASE_URL="${BASE_URL:-http://localhost:8080}"

echo "=========================================="
echo "Running k6 Concurrency Tests"
echo "=========================================="
echo "BASE_URL: $BASE_URL"
echo ""
echo "Note: Test data (orgs, units, forms) is created automatically during setup"
echo ""

# Test 1: Form Response Submission
if [ -n "$USER_UUIDS" ]; then
    echo "1. Testing Form Response Submission..."
    k6 run --env BASE_URL="$BASE_URL" \
           --env USER_UUIDS="$USER_UUIDS" \
           form-response-submission.js
    echo ""
else
    echo "1. Skipping Form Response Submission (missing USER_UUIDS)"
    echo ""
fi

# Test 2: JWT Refresh Token
if [ -n "$USER_UUIDS" ]; then
    echo "2. Testing JWT Refresh Token..."
    k6 run --env BASE_URL="$BASE_URL" \
           --env USER_UUIDS="$USER_UUIDS" \
           jwt-refresh-token.js
    echo ""
else
    echo "2. Skipping JWT Refresh Token (missing USER_UUIDS)"
    echo ""
fi

# Test 3: Form Publishing
if [ -n "$USER_UUIDS" ]; then
    echo "3. Testing Form Publishing..."
    k6 run --env BASE_URL="$BASE_URL" \
           --env USER_UUIDS="$USER_UUIDS" \
           form-publishing.js
    echo ""
else
    echo "3. Skipping Form Publishing (missing USER_UUIDS)"
    echo ""
fi

# Test 4: Unit Member Management
if [ -n "$ADMIN_UUIDS" ]; then
    echo "4. Testing Unit Member Management..."
    k6 run --env BASE_URL="$BASE_URL" \
           --env ADMIN_UUIDS="$ADMIN_UUIDS" \
           --env MEMBER_EMAILS="${MEMBER_EMAILS:-test@example.com}" \
           unit-member-management.js
    echo ""
else
    echo "4. Skipping Unit Member Management (missing ADMIN_UUIDS)"
    echo ""
fi

# Test 5: Slug Update Transaction
if [ -n "$USER_UUIDS" ]; then
    echo "5. Testing Slug Update Transaction..."
    k6 run --env BASE_URL="$BASE_URL" \
           --env USER_UUIDS="$USER_UUIDS" \
           slug-update-transaction.js
    echo ""
else
    echo "5. Skipping Slug Update Transaction (missing USER_UUIDS)"
    echo ""
fi

echo "=========================================="
echo "All tests completed!"
echo "=========================================="
