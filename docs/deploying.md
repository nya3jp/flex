# Deploying to Google Cloud Platform

```sh
# Settings
PROJECT=my-flex-project
REGION=my-region
FLEX_PASSWORD=my-password
DB_INSTANCE_NAME=sandbox
BUCKET_NAME="${PROJECT}-flex"

# Set current project
gcloud config set project "${PROJECT}"

# Enable required APIs
gcloud services enable iamcredentials.googleapis.com

# Create a service account
gcloud iam service-accounts create flexhub
gcloud projects add-iam-policy-binding "${PROJECT}" --member="serviceAccount:flexhub@${PROJECT}.iam.gserviceaccount.com" --role=roles/cloudsql.client
gcloud projects add-iam-policy-binding "${PROJECT}" --member="serviceAccount:flexhub@${PROJECT}.iam.gserviceaccount.com" --role=roles/iam.serviceAccountTokenCreator

# Push Docker images
docker pull ghcr.io/nya3jp/flexhub
docker tag ghcr.io/nya3jp/flexhub gcr.io/${PROJECT}/flexhub
docker push gcr.io/${PROJECT}/flexhub
docker pull ghcr.io/nya3jp/flexdash
docker tag ghcr.io/nya3jp/flexdash gcr.io/${PROJECT}/flexdash
docker push gcr.io/${PROJECT}/flexdash

# Create a MySQL instance
gcloud sql instances create "${DB_INSTANCE_NAME}" \
    --region="${REGION}" \
    --availability-type=zonal \
    --tier=db-f1-micro \
    --database-version=MYSQL_8_0
gcloud sql users create --instance="${DB_INSTANCE_NAME}" --host=% --password=flexhub flexhub
gcloud sql databases create flex --instance="${DB_INSTANCE_NAME}"

# Create a Storage bucket
gsutil mb -b on -l "${REGION}" "gs://${BUCKET_NAME}"
gsutil iam ch "serviceAccount:flexhub@${PROJECT}.iam.gserviceaccount.com:roles/storage.objectAdmin" "gs://${BUCKET_NAME}"

# Deploy Flexhub to Cloud Run
gcloud beta run deploy \
    --region="${REGION}" \
    --image="gcr.io/${PROJECT}/flexhub" \
    --args="--db=flexhub:flexhub@unix(/cloudsql/${PROJECT}:${REGION}:${DB_INSTANCE_NAME})/flex?parseTime=true,--fs=gs://${BUCKET_NAME}/,--password=${FLEX_PASSWORD}" \
    --ingress=all \
    --allow-unauthenticated \
    --min-instances=0 \
    --use-http2 \
    --service-account="flexhub@${PROJECT}.iam.gserviceaccount.com" \
    --add-cloudsql-instances="${PROJECT}:${REGION}:${DB_INSTANCE_NAME}" \
    flexhub

# Deploy Flexdash to Cloud Run
FLEXHUB_URL="$(gcloud run services describe flexhub --region="${REGION}" --format='value(status.url)')"
gcloud beta run deploy \
    --region="${REGION}" \
    --image="gcr.io/${PROJECT}/flexdash" \
    --args="--hub=${FLEXHUB_URL},--password=${FLEX_PASSWORD}" \
    --ingress=all \
    --allow-unauthenticated \
    --min-instances=0 \
    flexdash
```
