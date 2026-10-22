"""Access-control test of the Poro object storage (ADR-0015).

Runs against a SeaweedFS configured by infra/storage/init.sh.

Usage: python3 check_storage.py <endpoint> seed|matrix|rotated|expiry-seed|expiry

  seed     before init.sh: store the objects an earlier version left behind
           (SeaweedFS still accepts anonymous writes at that point).
  matrix   after init.sh: every identity reaches its own buckets only, and
           anonymous clients read public objects and nothing else.
           It also checks that storage-migrate moved the legacy video sources.
  rotated  after init.sh ran again with a new video secret: the old secret
           is refused, the new one works.
  expiry-seed  after init.sh ran with UPLOAD_TTL=1m: store image originals,
           an unfinished video multipart upload and a completed video source.
  expiry   once the TTL elapsed and janitor.sh ran with JANITOR_MAX_AGE=1m:
           the originals and the multipart upload are gone, the completed
           source is kept (PORO-008).

Credentials come from the S3_<SERVICE>_ACCESS_KEY_ID / _SECRET_ACCESS_KEY
variables that init.sh received; S3_VIDEO_OLD_SECRET_ACCESS_KEY holds the
video secret before rotation.
"""

import os
import sys
import urllib.error
import urllib.request
import uuid

import boto3
from botocore.config import Config
from botocore.exceptions import ClientError

PUBLIC = {"poro-videos", "poro-avatars", "poro-shop"}
OWNED = {
    "USER": {"poro-avatars", "poro-avatar-uploads"},
    "VIDEO": {"poro-videos", "poro-video-sources", "poro-quarantine"},
    "SHOP": {"poro-shop", "poro-shop-uploads"},
}
ALL = set().union(*OWNED.values())
LEGACY_UPLOADS = [("poro-avatars", "avatars/uploads/u1/legacy"), ("poro-shop", "uploads/o1/legacy")]
KEPT = [
    ("poro-avatars", "avatars/u1/kept.webp"),
    ("poro-shop", "shops/s1/kept.webp"),
    (
        "poro-videos",
        "videos/0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5a6b/0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5a6c/thumb.jpg",
    ),
]
# Sources an earlier version stored next to the public media and in quarantine.
LEGACY_SOURCES = [
    (
        "poro-videos",
        "videos/0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5a6b/0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5a6c/source.mp4",
    ),
    (
        "poro-quarantine",
        "videos/0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5a6b/0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5a6d/source.mov",
    ),
]


def client(endpoint, access_key, secret_key):
    return boto3.client(
        "s3",
        endpoint_url=endpoint,
        aws_access_key_id=access_key,
        aws_secret_access_key=secret_key,
        region_name="us-east-1",
        config=Config(
            signature_version="s3v4",
            s3={"addressing_style": "path"},
            retries={"max_attempts": 1},
            request_checksum_calculation="when_required",
            response_checksum_validation="when_required",
        ),
    )


def service(endpoint, name, secret=None):
    return client(
        endpoint,
        os.environ[f"S3_{name}_ACCESS_KEY_ID"],
        secret or os.environ[f"S3_{name}_SECRET_ACCESS_KEY"],
    )


def anonymous(endpoint, method, path, body=None):
    req = urllib.request.Request(endpoint + path, method=method, data=body)
    try:
        with urllib.request.urlopen(req, timeout=10) as resp:
            return resp.status
    except urllib.error.HTTPError as err:
        return err.code


DENIED = ("AccessDenied", "SignatureDoesNotMatch", "InvalidAccessKeyId")


def allowed(call, **params):
    """Whether the store let the call through its access control.

    Any error other than a denial (a missing key, a bucket that already
    exists...) comes from past the access check, so it counts as allowed.
    """
    try:
        call(**params)
        return True
    except ClientError as err:
        return err.response["Error"]["Code"] not in DENIED


def expect(failures, condition, message):
    if not condition:
        failures.append(message)


def seed(endpoint):
    for bucket, key in LEGACY_UPLOADS + KEPT + LEGACY_SOURCES:
        anonymous(endpoint, "PUT", f"/{bucket}")
        status = anonymous(endpoint, "PUT", f"/{bucket}/{key}", b"legacy")
        if status != 200:
            sys.exit(f"seed {bucket}/{key}: {status}")
    print(f"storage: seeded {len(LEGACY_UPLOADS + KEPT + LEGACY_SOURCES)} objects before init")


def check_services(endpoint, failures):
    marker = uuid.uuid4().hex
    for name, owned in OWNED.items():
        s3 = service(endpoint, name)
        for bucket in sorted(ALL):
            key = f"check/{name.lower()}-{marker}"
            mine = bucket in owned
            wrote = allowed(s3.put_object, Bucket=bucket, Key=key, Body=b"x")
            expect(failures, wrote == mine, f"{name} write {bucket}: allowed={wrote}, want {mine}")
            listed = allowed(s3.list_objects_v2, Bucket=bucket, MaxKeys=1)
            expect(failures, listed == mine, f"{name} list {bucket}: allowed={listed}, want {mine}")
        for bucket in sorted(ALL - owned):
            read = allowed(s3.get_object, Bucket=bucket, Key=f"check/{marker}")
            expect(failures, not read, f"{name} read another service's bucket {bucket}")
        bucket = sorted(owned)[0]
        expect(
            failures,
            not allowed(s3.delete_bucket, Bucket=bucket),
            f"{name} deleted bucket {bucket}",
        )
        expect(
            failures,
            not allowed(s3.create_bucket, Bucket=f"rogue-{marker[:8]}"),
            f"{name} created a bucket",
        )


def check_anonymous(endpoint, failures):
    video = service(endpoint, "VIDEO")
    marker = uuid.uuid4().hex
    for bucket in sorted(ALL):
        owner = next(name for name, owned in OWNED.items() if bucket in owned)
        key = f"anon/{marker}.bin"
        service(endpoint, owner).put_object(Bucket=bucket, Key=key, Body=b"x")
        want = 200 if bucket in PUBLIC else 403
        got = anonymous(endpoint, "GET", f"/{bucket}/{key}")
        expect(failures, got == want, f"anonymous GET {bucket}/{key}: {got}, want {want}")
        got = anonymous(endpoint, "GET", f"/{bucket}?list-type=2")
        expect(failures, got == 403, f"anonymous list {bucket}: {got}, want 403")
        got = anonymous(endpoint, "PUT", f"/{bucket}/anon-{marker}", b"y")
        expect(failures, got == 403, f"anonymous PUT {bucket}: {got}, want 403")
        got = anonymous(endpoint, "DELETE", f"/{bucket}/{key}")
        expect(failures, got == 403, f"anonymous DELETE {bucket}/{key}: {got}, want 403")
    got = anonymous(endpoint, "PUT", f"/rogue-{marker[:8]}")
    expect(failures, got == 403, f"anonymous bucket creation: {got}, want 403")
    got = anonymous(endpoint, "GET", "/")
    expect(failures, got == 403, f"anonymous bucket listing: {got}, want 403")

    # The video source of the pipeline: a presigned multipart part, as the app sends it.
    source = f"videos/{uuid.uuid4()}/{uuid.uuid4()}/source.mp4"
    upload = video.create_multipart_upload(Bucket="poro-video-sources", Key=source, ContentType="video/mp4")
    url = video.generate_presigned_url(
        "upload_part",
        Params={
            "Bucket": "poro-video-sources",
            "Key": source,
            "UploadId": upload["UploadId"],
            "PartNumber": 1,
        },
        ExpiresIn=600,
    )
    req = urllib.request.Request(url, data=b"ftypisom", method="PUT")
    with urllib.request.urlopen(req, timeout=10) as resp:
        etag = resp.headers["ETag"]
    video.complete_multipart_upload(
        Bucket="poro-video-sources",
        Key=source,
        UploadId=upload["UploadId"],
        MultipartUpload={"Parts": [{"ETag": etag, "PartNumber": 1}]},
    )
    got = anonymous(endpoint, "GET", f"/poro-video-sources/{source}")
    expect(failures, got == 403, f"anonymous GET of a video source: {got}, want 403")
    got = anonymous(endpoint, "GET", f"/poro-videos/{source}")
    expect(failures, got == 404, f"a video source reached the public bucket: {got}")

    # An avatar original: a presigned POST into the private upload bucket.
    user = service(endpoint, "USER")
    key = f"avatars/uploads/{uuid.uuid4()}/{uuid.uuid4()}"
    post = user.generate_presigned_post(
        "poro-avatar-uploads",
        key,
        Fields={"Content-Type": "image/png"},
        Conditions=[{"Content-Type": "image/png"}, ["content-length-range", 1, 1024]],
        ExpiresIn=300,
    )
    boundary = uuid.uuid4().hex
    body = b""
    for name, value in post["fields"].items():
        body += f'--{boundary}\r\nContent-Disposition: form-data; name="{name}"\r\n\r\n{value}\r\n'.encode()
    body += (
        (
            f'--{boundary}\r\nContent-Disposition: form-data; name="file"; filename="a.png"\r\n'
            f"Content-Type: image/png\r\n\r\n"
        ).encode()
        + b"\x89PNG"
        + f"\r\n--{boundary}--\r\n".encode()
    )
    req = urllib.request.Request(
        post["url"],
        data=body,
        method="POST",
        headers={"Content-Type": f"multipart/form-data; boundary={boundary}"},
    )
    with urllib.request.urlopen(req, timeout=10) as resp:
        expect(failures, resp.status in (200, 201, 204), f"presigned POST of an avatar: {resp.status}")
    got = anonymous(endpoint, "GET", f"/poro-avatar-uploads/{key}")
    expect(failures, got == 403, f"anonymous GET of an avatar original: {got}, want 403")


def check_cleanup(endpoint, failures):
    for bucket, key in LEGACY_UPLOADS:
        got = anonymous(endpoint, "GET", f"/{bucket}/{key}")
        expect(failures, got == 404, f"legacy raw upload {bucket}/{key} still served: {got}")
    for bucket, key in KEPT:
        got = anonymous(endpoint, "GET", f"/{bucket}/{key}")
        expect(failures, got == 200, f"published media {bucket}/{key} lost: {got}")


def check_migrated(endpoint, failures):
    video = service(endpoint, "VIDEO")
    for bucket, key in LEGACY_SOURCES:
        got = anonymous(endpoint, "GET", f"/{bucket}/{key}")
        expect(failures, got in (403, 404), f"legacy source {bucket}/{key} still served: {got}")
        got = anonymous(endpoint, "GET", f"/poro-video-sources/{key}")
        expect(failures, got == 403, f"anonymous GET of a migrated source: {got}, want 403")
        try:
            body = video.get_object(Bucket="poro-video-sources", Key=key)["Body"].read()
            expect(failures, body == b"legacy", f"migrated source {key} has the wrong content")
        except ClientError as err:
            failures.append(f"migrated source {key} missing from poro-video-sources: {err}")


def check_rotation(endpoint, failures):
    old = service(endpoint, "VIDEO", secret=os.environ["S3_VIDEO_OLD_SECRET_ACCESS_KEY"])
    new = service(endpoint, "VIDEO")
    expect(
        failures,
        not allowed(old.list_objects_v2, Bucket="poro-videos", MaxKeys=1),
        "the old video secret still works",
    )
    expect(
        failures,
        allowed(new.list_objects_v2, Bucket="poro-videos", MaxKeys=1),
        "the new video secret is refused",
    )


EXPIRING = [("USER", "poro-avatar-uploads", "avatars/uploads/u1/expiring"), ("SHOP", "poro-shop-uploads", "uploads/o1/expiring")]
SOURCE = "videos/u1/v1/source.mp4"
UNFINISHED = "videos/u1/v2/source.mp4"


def expiry_seed(endpoint):
    for name, bucket, key in EXPIRING:
        service(endpoint, name).put_object(Bucket=bucket, Key=key, Body=b"original")
    video = service(endpoint, "VIDEO")
    video.put_object(Bucket="poro-video-sources", Key=SOURCE, Body=b"completed source")
    upload = video.create_multipart_upload(Bucket="poro-video-sources", Key=UNFINISHED)
    video.upload_part(
        Bucket="poro-video-sources", Key=UNFINISHED, UploadId=upload["UploadId"], PartNumber=1, Body=b"part"
    )


def check_expiry(endpoint, failures):
    for name, bucket, key in EXPIRING:
        listed = service(endpoint, name).list_objects_v2(Bucket=bucket).get("Contents", [])
        expect(failures, all(o["Key"] != key for o in listed), f"{bucket}/{key} outlived its TTL")
    video = service(endpoint, "VIDEO")
    uploads = video.list_multipart_uploads(Bucket="poro-video-sources").get("Uploads", [])
    expect(failures, all(u["Key"] != UNFINISHED for u in uploads), "the abandoned multipart upload was not aborted")
    expect(
        failures,
        allowed(video.head_object, Bucket="poro-video-sources", Key=SOURCE),
        "a completed video source was deleted",
    )


def main():
    endpoint, phase = sys.argv[1], sys.argv[2]
    if phase == "seed":
        seed(endpoint)
        return
    if phase == "expiry-seed":
        expiry_seed(endpoint)
        return
    phases = {
        "matrix": [check_services, check_anonymous, check_cleanup, check_migrated],
        "rotated": [check_rotation, check_anonymous],
        "expiry": [check_expiry],
    }
    if phase not in phases:
        sys.exit(f"unknown phase {phase}")
    failures = []
    for check in phases[phase]:
        # A store without access control lets one check break the next one's
        # setup: report that instead of stopping at the first traceback.
        try:
            check(endpoint, failures)
        except (ClientError, OSError) as err:
            failures.append(f"{check.__name__} could not run: {err}")
    if failures:
        print(f"{len(failures)} storage check(s) failed:")
        for failure in failures:
            print(f"  - {failure}")
        sys.exit(1)
    print(f"storage: {phase} checks out")


if __name__ == "__main__":
    main()
