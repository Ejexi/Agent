---
name: aws-security
description: AWS misconfigurations — IAM privilege escalation, S3 exposure, metadata service, Lambda, EKS, CloudTrail gaps
---

# AWS Security

AWS misconfigurations are a top source of breaches. Overly permissive IAM, exposed S3 buckets, and SSRF to the metadata service are the most common attack paths.

## IAM Privilege Escalation

### Common Escalation Paths

```
PassRole + CreateFunction → attach admin role to Lambda → call Lambda → admin
iam:CreateAccessKey on any user → create key for high-priv user
iam:AttachUserPolicy → attach AdministratorAccess to self
iam:CreatePolicyVersion → update existing policy to add * permissions
iam:SetDefaultPolicyVersion → revert to old permissive version
sts:AssumeRole on * → assume any role in the account
```

### Over-permissive Policies (checkov findings)

```json
{
  "Effect": "Allow",
  "Action": "*",          // ❌ wildcard action
  "Resource": "*"         // ❌ wildcard resource
}

{
  "Effect": "Allow",
  "Action": "iam:*",      // ❌ full IAM control = admin
  "Resource": "*"
}
```

### Least-Privilege Patterns

```json
{
  "Effect": "Allow",
  "Action": [
    "s3:GetObject",
    "s3:PutObject"
  ],
  "Resource": "arn:aws:s3:::my-bucket/*",  // ✅ specific resource
  "Condition": {
    "StringEquals": {
      "s3:prefix": ["uploads/"]            // ✅ condition
    }
  }
}
```

## S3 Security

```bash
# Check bucket ACL
aws s3api get-bucket-acl --bucket target-bucket

# Check bucket policy
aws s3api get-bucket-policy --bucket target-bucket

# List without auth (public bucket)
aws s3 ls s3://target-bucket --no-sign-request

# Common misconfig: AllUsers or AuthenticatedUsers in ACL
```

**IaC checklist (checkov):**
```
✓ S3 Block Public Access: all 4 settings enabled
✓ Bucket versioning enabled
✓ Server-side encryption enabled (AES-256 or aws:kms)
✓ Bucket logging enabled
✓ No wildcard in bucket policy Principal
```

## SSRF to Metadata Service (IMDSv1)

```bash
# Classic SSRF target — always try when SSRF is found
curl http://169.254.169.254/latest/meta-data/
curl http://169.254.169.254/latest/meta-data/iam/security-credentials/
curl http://169.254.169.254/latest/meta-data/iam/security-credentials/ROLE_NAME

# Returns temporary credentials:
{
  "AccessKeyId": "ASIA...",
  "SecretAccessKey": "...",
  "Token": "...",
  "Expiration": "..."
}
```

**IMDSv2 (required hop limit = 1, token required):**
```bash
TOKEN=$(curl -X PUT "http://169.254.169.254/latest/api/token" \
  -H "X-aws-ec2-metadata-token-ttl-seconds: 21600")
curl -H "X-aws-ec2-metadata-token: $TOKEN" \
  http://169.254.169.254/latest/meta-data/
```

**IaC: enforce IMDSv2:**
```hcl
metadata_options {
  http_endpoint               = "enabled"
  http_tokens                 = "required"  # ✅ IMDSv2
  http_put_response_hop_limit = 1
}
```

## Lambda Security

```python
# ❌ Secrets in environment variables (visible in console, CloudTrail)
import os
db_password = os.environ['DB_PASSWORD']  # stored in plain text in Lambda config

# ✅ Use Secrets Manager or Parameter Store
import boto3
client = boto3.client('secretsmanager')
secret = client.get_secret_value(SecretId='prod/db/password')

# ❌ Overpermissive Lambda execution role
# Role has AdministratorAccess — Lambda can do anything if compromised
```

## CloudTrail Gaps

Missing or misconfigured CloudTrail:
```
✗ No CloudTrail enabled → no audit log
✗ CloudTrail not in all regions → blind spots
✗ Log validation disabled → logs can be tampered
✗ No CloudWatch alarm on root login
✗ No alarm on ConsoleLoginFailure
```

**Key events to alert on:**
```
root account login
IAM user/key creation
Security group changes
S3 bucket policy changes
CloudTrail disabled/modified
```

## EKS / Kubernetes on AWS

```bash
# Service account with IRSA → can call AWS APIs
# If pod is compromised → can use the SA's AWS permissions

# Check IRSA annotation
kubectl get sa -n default -o yaml | grep amazonaws.com/role-arn

# IMDSv1 from pod (if not blocked by network policy)
curl http://169.254.169.254/latest/meta-data/iam/security-credentials/
```

## Checkov / tfsec Rules for AWS

```
CKV_AWS_1   - IAM policy no wildcard action
CKV_AWS_18  - S3 logging enabled
CKV_AWS_19  - S3 encryption enabled
CKV_AWS_20  - S3 not public
CKV_AWS_21  - S3 versioning enabled
CKV_AWS_79  - IMDSv2 required on EC2
CKV_AWS_111 - IAM no admin privileges
CKV_AWS_115 - Lambda not public
```
