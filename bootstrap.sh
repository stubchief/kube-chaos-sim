#!/usr/bin/env bash
# bootstrap.sh — one-time manual setup for kube-chaos-sim cloud deployment.
#
# Run this ONCE from your local machine before the first deploy.yml run.
# It is safe to re-run: steps that are already done will just report that
# and move on (idempotent where Yandex Cloud / Terraform allow it).
#
# Prerequisites: yc CLI installed and authenticated (yc init already done).

set -euo pipefail

FOLDER_ID="$(yc config get folder-id)"
if [ -z "$FOLDER_ID" ]; then
  echo "No folder-id set in yc config. Run 'yc init' first."
  exit 1
fi
echo "Using folder_id: $FOLDER_ID"

# ---------------------------------------------------------------------------
# Step 1 — Terraform state bucket (terraform/bootstrap)
# ---------------------------------------------------------------------------
echo
echo "=== Step 1: Terraform state bucket ==="
export TF_VAR_folder_id="$FOLDER_ID"
export YC_TOKEN="$(yc iam create-token)"

pushd "$(dirname "$0")/terraform/bootstrap" > /dev/null
terraform init -input=false
terraform apply -auto-approve
popd > /dev/null

# ---------------------------------------------------------------------------
# Step 2 — CI service account + static keys (for GitHub Secrets)
# ---------------------------------------------------------------------------
echo
echo "=== Step 2: CI service account ==="
SA_NAME="kube-chaos-sim-ci"

if yc iam service-account get --name "$SA_NAME" > /dev/null 2>&1; then
  echo "Service account '$SA_NAME' already exists, skipping creation."
else
  yc iam service-account create --name "$SA_NAME"
fi

SA_ID="$(yc iam service-account get --name "$SA_NAME" --format json | grep -o '"id": *"[^"]*"' | head -1 | cut -d'"' -f4)"
echo "Service account id: $SA_ID"

echo "Granting 'editor' role on folder to $SA_NAME (idempotent, safe to re-run)..."
yc resource-manager folder add-access-binding "$FOLDER_ID" \
  --role editor \
  --subject "serviceAccount:$SA_ID"

echo
echo ">>> If you haven't already, create the IAM key and static access key"
echo ">>> and put them in GitHub Secrets. This step is NOT idempotent —"
echo ">>> only run it once, the private key/secret is shown only at creation:"
echo
echo "    yc iam key create --service-account-id $SA_ID --output key.json"
echo "    yc iam access-key create --service-account-id $SA_ID"
echo
read -p "Have you already created and saved these to GitHub Secrets? [y/N] " ANSWER
if [ "$ANSWER" != "y" ] && [ "$ANSWER" != "Y" ]; then
  echo "Run the two commands above manually, save the outputs to GitHub Secrets:"
  echo "  YC_KEY_JSON            <- full contents of key.json"
  echo "  AWS_ACCESS_KEY_ID      <- key_id from access-key create output"
  echo "  AWS_SECRET_ACCESS_KEY  <- secret from access-key create output"
  echo "Then re-run this script, answer 'y' at this prompt, and it will continue."
  exit 0
fi

# main.tf's provider "yandex" block expects a key file at /tmp/key.json —
# that's how GitHub Actions provides it (written from the YC_KEY_JSON
# secret just before terraform runs). Locally, no such step has run, so
# we need the actual key.json you created above, in the same place.
if [ ! -s /tmp/key.json ]; then
  echo
  if [ -s "$(dirname "$0")/key.json" ]; then
    echo "Found key.json in repo root, copying to /tmp/key.json"
    cp "$(dirname "$0")/key.json" /tmp/key.json
  else
    echo "ERROR: /tmp/key.json not found or empty."
    echo "Re-run 'yc iam key create --service-account-id $SA_ID --output key.json'"
    echo "then either move it to /tmp/key.json, or place it as key.json next to"
    echo "this script and re-run."
    exit 1
  fi
fi

# ---------------------------------------------------------------------------
# Step 3 — Import the CI service account into the main Terraform state
# ---------------------------------------------------------------------------
echo
echo "=== Step 3: Import CI service account into main Terraform state ==="
echo "This makes 'terraform apply' in CI aware that this service account"
echo "already exists, instead of trying (and failing) to create it again."

pushd "$(dirname "$0")/terraform" > /dev/null
terraform init -input=false -backend-config="bucket=chaos-sim-tf-state"

if terraform state list | grep -q "yandex_iam_service_account.ci"; then
  echo "Already imported, skipping."
else
  terraform import -var="folder_id=$FOLDER_ID" yandex_iam_service_account.ci "$SA_ID"
fi
popd > /dev/null

echo
echo "=== Bootstrap complete ==="
echo "You can now trigger deploy.yml from GitHub Actions."
echo "Remember to verify these GitHub Secrets exist:"
echo "  YC_KEY_JSON, YC_FOLDER_ID, AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY,"
echo "  TF_STATE_BUCKET (= chaos-sim-tf-state)"