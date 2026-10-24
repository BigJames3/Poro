"""End-to-end journeys of the Android app against the whole running stack.

Every request goes through the gateway, the way the app sends it. Each journey
checks status codes and the data the next step needs. At the end the script
compares the routes it exercised with the routes the app calls (parsed from its
Retrofit interfaces) and fails under the required coverage.

Usage: python3 tests/e2e/e2e.py
Environment:
  E2E_BASE_URL      gateway, default http://127.0.0.1:8000
  E2E_AUTH_LOGS     shell command printing the auth service logs (dev SMS and
                    email codes are logged there), default
                    "docker compose logs --no-log-prefix auth"
  E2E_MIN_COVERAGE  minimum share of the app's routes, default 0.8
"""
import json
import os
import re
import subprocess
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
import uuid
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
FIXTURES = Path(__file__).resolve().parent / "fixtures"
BASE = os.environ.get("E2E_BASE_URL", "http://127.0.0.1:8000").rstrip("/")
AUTH_LOGS = os.environ.get("E2E_AUTH_LOGS", "docker compose logs --no-log-prefix auth")
MIN_COVERAGE = float(os.environ.get("E2E_MIN_COVERAGE", "0.8"))
RUN = uuid.uuid4().hex[:8]

covered = set()  # (method, template) answered as expected


class Failure(Exception):
    pass


def check(condition, message):
    if not condition:
        raise Failure(message)


def wait_for(what, probe, timeout=90, interval=1.5):
    """Polls probe() until it returns a truthy value: events cross Redpanda."""
    deadline = time.time() + timeout
    while True:
        value = probe()
        if value:
            return value
        if time.time() > deadline:
            raise Failure(f"timed out waiting for {what}")
        time.sleep(interval)


def http(method, url, body=None, headers=None, raw=False):
    data = None
    headers = dict(headers or {})
    if body is not None and not raw:
        data = json.dumps(body).encode()
        headers["Content-Type"] = "application/json"
    elif raw:
        data = body
    req = urllib.request.Request(url, data=data, method=method, headers=headers)
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            return resp.status, resp.read(), dict(resp.headers)
    except urllib.error.HTTPError as err:
        return err.code, err.read(), dict(err.headers)


class Client:
    def __init__(self, name):
        self.name = name
        self.token = None
        self.refresh_token = None
        self.user = None

    def call(self, method, template, path=None, body=None, query=None, expect=(200,), auth=True, headers=None):
        """Calls the API. template is the route as the app declares it; path fills it."""
        url = BASE + (path or template.split("?")[0])
        params = dict(query or {})
        if "?" in template:
            params.update(urllib.parse.parse_qsl(template.split("?", 1)[1]))
        if params:
            url += "?" + urllib.parse.urlencode(params)
        headers = dict(headers or {})
        if auth and self.token:
            headers["Authorization"] = f"Bearer {self.token}"
        status, raw, _ = http(method, url, body, headers)
        try:
            envelope = json.loads(raw) if raw else {}
        except ValueError:
            envelope = {"raw": raw[:200]}
        if isinstance(expect, int):
            expect = (expect,)
        check(status in expect, f"{self.name}: {method} {path or template} -> {status}, expected {expect}: {str(envelope)[:400]}")
        covered.add((method, template))
        return envelope.get("data") if isinstance(envelope, dict) else None


# ---------------------------------------------------------------- codes

def auth_logs():
    return subprocess.run(AUTH_LOGS, shell=True, cwd=ROOT, capture_output=True, text=True).stdout


def log_lines(marker):
    return [line for line in auth_logs().splitlines() if marker in line]


def next_code(marker, before, what):
    """First code logged after the before-th line carrying marker (contacts are redacted)."""
    def probe():
        lines = log_lines(marker)
        if len(lines) <= before:
            return None
        found = re.findall(r'"code":\s*"(\d+)"', lines[before]) or re.findall(r"\b(\d{6})\b", lines[before])
        return found[-1] if found else None
    return wait_for(what, probe, timeout=20, interval=0.5)


SMS = "dev sms otp"
EMAIL = "dev email"


# ---------------------------------------------------------------- journeys

def otp_login(client, phone):
    before = len(log_lines(SMS))
    client.call("POST", "/api/v1/auth/otp/request", body={"phone": phone}, auth=False)
    code = next_code(SMS, before, f"SMS code of {client.name}")
    data = client.call("POST", "/api/v1/auth/otp/verify", auth=False,
                       body={"phone": phone, "code": code, "device_name": f"e2e {client.name}"})
    client.token, client.refresh_token, client.user = data["access_token"], data["refresh_token"], data["user"]
    return data


def guest():
    anon = Client("guest")
    page = anon.call("GET", "/api/v1/feed/public/trending", auth=False)
    check("items" in page, "public trending without items")
    for path in ("/api/v1/feed/for-you", "/api/v1/feed/following"):
        status, _, _ = http("GET", BASE + path)
        check(status == 401, f"guest {path} -> {status}, expected 401")


def account(seller):
    me = seller.call("GET", "/api/v1/auth/me")
    check(me["id"] == seller.user["id"], "auth/me returns another account")
    data = seller.call("POST", "/api/v1/auth/refresh", body={"refresh_token": seller.refresh_token}, auth=False)
    seller.token, seller.refresh_token = data["access_token"], data["refresh_token"]


def email_account():
    """Email sign-up and verification, sessions on two devices."""
    email, password = f"e2e-{RUN}@poro.test", f"Mot-de-passe-{RUN}"
    carol = Client("carol")
    before = len(log_lines(EMAIL))
    data = carol.call("POST", "/api/v1/auth/email/register", expect=201, auth=False,
                      body={"email": email, "password": password, "full_name": "Carol E2E", "device_name": "e2e phone"})
    carol.token, carol.refresh_token, carol.user = data["access_token"], data["refresh_token"], data["user"]
    carol.email, carol.password, carol.registered_at = email, password, time.time()
    # Sign-up emails the first code; asking again once verified is refused.
    code = next_code(EMAIL, before, "email verification code")
    carol.call("POST", "/api/v1/auth/email/verification/confirm", body={"code": code}, expect=(200, 204))
    check(carol.call("GET", "/api/v1/auth/me")["email_verified"] is True, "email not verified")
    carol.call("POST", "/api/v1/auth/email/verification/request", expect=409)

    tablet = email_login(carol, "carol-tablet")
    sessions = carol.call("GET", "/api/v1/auth/sessions")
    items = sessions["items"] if isinstance(sessions, dict) else sessions
    check(len(items) >= 2, f"{len(items)} sessions, expected 2")
    others = [x for x in items if not x.get("current")]
    carol.call("DELETE", "/api/v1/auth/sessions/{id}", f"/api/v1/auth/sessions/{others[0]['id']}", expect=(200, 204))
    check(status_of(tablet) == 401, "deleted session still accepted")
    laptop = email_login(carol, "carol-laptop")
    carol.call("POST", "/api/v1/auth/sessions/revoke-others", expect=(200, 204))
    check(status_of(laptop) == 401, "revoked session still accepted")
    return carol


def email_login(account, name):
    c = Client(name)
    d = c.call("POST", "/api/v1/auth/email/login", auth=False,
               body={"email": account.email, "password": account.password, "device_name": f"e2e {name}"})
    c.token, c.refresh_token, c.user = d["access_token"], d["refresh_token"], d["user"]
    return c


def status_of(client):
    return http("GET", BASE + "/api/v1/auth/me", headers={"Authorization": f"Bearer {client.token}"})[0]


def goodbye(carol):
    """Password reset, logout of the old device, then deletion of the account."""
    # One email per address every 60 s: the sign-up email was the last one.
    time.sleep(max(0.0, carol.registered_at + 62 - time.time()))
    before = len(log_lines(EMAIL))
    carol.call("POST", "/api/v1/auth/email/password/forgot", body={"email": carol.email}, auth=False, expect=202)
    code = next_code(EMAIL, before, "password reset code")
    carol.password += "-2"
    carol.call("POST", "/api/v1/auth/email/password/reset", auth=False, expect=(200, 204),
               body={"email": carol.email, "code": code, "new_password": carol.password})
    old_phone = email_login(carol, "carol-old-phone")
    old_phone.call("POST", "/api/v1/auth/logout", expect=(200, 204))
    check(status_of(old_phone) == 401, "logged out session still accepted")
    # Deleting the account signs every device out.
    fresh = email_login(carol, "carol-again")
    fresh.call("POST", "/api/v1/auth/me/delete", body={"password": carol.password}, expect=(200, 202, 204))
    check(status_of(fresh) == 401, "session of a deleted account still accepted")


def profile(client, username):
    me = client.call("GET", "/api/v1/users/me")
    check(client.user["id"] in json.dumps(me), "users/me of another account")
    avail = client.call("GET", "/api/v1/users/username-availability", query={"username": username})
    check(avail.get("available") is True, f"username {username} not available: {avail}")
    client.call("PATCH", "/api/v1/users/me", body={"username": username, "display_name": f"E2E {client.name}", "bio": "Parcours de bout en bout"})
    profile = client.call("GET", "/api/v1/users/by-username/{username}", f"/api/v1/users/by-username/{username}")
    check(profile["username"] == username, "by-username returns another profile")
    users = client.call("GET", "/api/v1/users", query={"ids": client.user["id"]})
    check(len(users["items"] if isinstance(users, dict) else users) == 1, "users?ids= did not return the profile")
    return profile


def presigned_image(client, url_template, url_path, confirm_template, confirm_path):
    """Asks for a presigned form, posts the image to storage, confirms it."""
    form = client.call("POST", url_template, url_path, body={"content_type": "image/jpeg"}, expect=(200, 201))
    image = (FIXTURES / "image.jpg").read_bytes()
    boundary = "e2e" + uuid.uuid4().hex
    parts = []
    for name, value in form["fields"].items():
        parts.append(f'--{boundary}\r\nContent-Disposition: form-data; name="{name}"\r\n\r\n{value}\r\n'.encode())
    parts.append(f'--{boundary}\r\nContent-Disposition: form-data; name="file"; filename="e2e.jpg"\r\n'
                 f"Content-Type: image/jpeg\r\n\r\n".encode() + image + b"\r\n")
    parts.append(f"--{boundary}--\r\n".encode())
    status, raw, _ = http("POST", form["upload_url"], b"".join(parts),
                          {"Content-Type": f"multipart/form-data; boundary={boundary}"}, raw=True)
    check(status in (200, 201, 204), f"storage refused the image: {status} {raw[:200]}")
    return client.call("PUT", confirm_template, confirm_path, body={"upload_key": form["upload_key"]})


def avatar(client):
    presigned_image(client, "/api/v1/users/me/avatar/upload-url", None, "/api/v1/users/me/avatar", None)
    me = client.call("GET", "/api/v1/users/me")
    check(me.get("avatar_url"), "avatar not set after confirmation")
    client.call("DELETE", "/api/v1/users/me/avatar", expect=(200, 204))


def upload_video(client, title, description):
    video = (FIXTURES / "video.mp4").read_bytes()
    init = client.call("POST", "/api/v1/videos/uploads", expect=(200, 201), body={
        "title": title, "description": description, "filename": "e2e.mp4",
        "content_type": "video/mp4", "size_bytes": len(video)})
    size = init["part_size"]
    etags = []
    for part in init["parts"]:
        n = part["part_number"]
        chunk = video[(n - 1) * size:n * size]
        status, raw, headers = http("PUT", part["url"], chunk, {}, raw=True)
        check(status == 200, f"storage refused part {n}: {status} {raw[:200]}")
        etags.append({"part_number": n, "etag": headers.get("ETag") or headers.get("Etag")})
    client.call("POST", "/api/v1/videos/uploads/{id}/complete", f"/api/v1/videos/uploads/{init['video_id']}/complete",
                body={"parts": etags}, expect=(200, 202))
    return init["video_id"]


PHOTOS = {"Poro-Capabilities": "photo-posts"}
# Magic bytes are all step 2b checks (ADR-0020); decoding comes with step 2c.
JPEG = bytes([0xFF, 0xD8, 0xFF, 0xE0, 0, 0x10]) + b"JFIF" + bytes(50)
PNG = b"\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR" + bytes(50)
WEBP = b"RIFF\x24\x00\x00\x00WEBPVP8 " + bytes(50)
HEIC = b"\x00\x00\x00\x18ftypheic\x00\x00\x00\x00mif1heic" + bytes(50)


def photo_post(client, images, caption=None):
    """Creates a photo post and returns it with its signed PUTs."""
    body = {"images": [{"content_type": ct, "size_bytes": len(raw)} for ct, raw in images]}
    if caption is not None:
        body["description"] = caption
    return client.call("POST", "/api/v1/videos/photo-uploads", body=body, expect=201)


def put_image(upload, raw, content_type=None):
    headers = dict(upload["headers"])
    headers.pop("Content-Length", None)  # urllib sends the body's own length
    if content_type is not None:
        headers["Content-Type"] = content_type
    status, _, _ = http("PUT", upload["url"], raw, headers, raw=True)
    return status


def photo_posts_closed(seller):
    """By default (PHOTO_POSTS_ENABLED=false) photo posts accept nothing, before
    authentication, and the video routes are untouched (ADR-0020)."""
    body = {"images": [{"content_type": "image/jpeg", "size_bytes": len(JPEG)}]}
    for client in (seller, Client("anonymous")):
        for path in ("/api/v1/videos/photo-uploads", f"/api/v1/videos/photo-uploads/{uuid.uuid4()}/complete"):
            status, raw, _ = http("POST", BASE + path, body, {"Authorization": f"Bearer {client.token}"} if client.token else {})
            code = json.loads(raw or b"{}").get("error", {}).get("code")
            check(status == 404 and code == "photo_posts_disabled", f"{path} while disabled: {status} {raw[:200]}")
    plain = seller.call("GET", "/api/v1/videos", query={"limit": 50}, headers=PHOTOS)
    check(all(v["media_type"] == "video" for v in plain["items"]), "a photo post exists while photo posts are disabled")


def photo_posts(seller):
    """Photo posts in the video service (step 2b), with PHOTO_POSTS_ENABLED=true:
    signed PUTs to the real storage, content checked at complete, no photo for a
    client without the capability."""
    images = [("image/jpeg", JPEG), ("image/png", PNG), ("image/webp", WEBP)]
    init = photo_post(seller, images, "Marché de Treichville")
    pid = init["video_id"]
    check(len(init["images"]) == 3 and init["expires_in"] == 3600, f"photo upload session: {init}")
    first = init["images"][0]
    check(put_image(first, JPEG, "image/png") == 403, "storage accepted another Content-Type than signed")
    check(put_image(first, JPEG + b"\x00") == 403, "storage accepted another length than signed")
    for upload, (_, raw) in zip(init["images"][:2], images):
        check(put_image(upload, raw) == 200, "storage refused the signed PUT")
    complete = ("POST", "/api/v1/videos/photo-uploads/{id}/complete", f"/api/v1/videos/photo-uploads/{pid}/complete")
    seller.call(*complete, expect=400)  # third image missing: nothing deleted
    check(put_image(init["images"][2], WEBP) == 200, "storage refused the third image")
    post = seller.call(*complete)
    check(post["media_type"] == "photo" and post["status"] == "processing", f"completed photo post: {post}")
    seller.call(*complete, expect=409)  # a replayed complete changes nothing

    get = ("GET", "/api/v1/videos/{id}", f"/api/v1/videos/{pid}")
    seller.call(*get, expect=404)  # no capability: not even its owner sees it
    check(seller.call(*get, headers=PHOTOS)["media_type"] == "photo", "photo post not returned with the capability")
    plain = seller.call("GET", "/api/v1/videos", query={"limit": 50})
    check(all(v["media_type"] == "video" for v in plain["items"]), "a photo post leaked to a client without the capability")
    mine = seller.call("GET", "/api/v1/videos", query={"limit": 50}, headers=PHOTOS)
    check(any(v["id"] == pid for v in mine["items"]), "photo post missing with the capability")

    bad = photo_post(seller, [("image/jpeg", JPEG), ("image/jpeg", HEIC)])
    for upload, raw in zip(bad["images"], (JPEG, HEIC)):
        check(put_image(upload, raw) == 200, "storage refused a signed PUT")
    seller.call("POST", "/api/v1/videos/photo-uploads/{id}/complete", f"/api/v1/videos/photo-uploads/{bad['video_id']}/complete", expect=422)
    seller.call("GET", "/api/v1/videos/{id}", f"/api/v1/videos/{bad['video_id']}", headers=PHOTOS, expect=404)  # no partial post

    for body, status in (({"images": []}, 422), ({"images": [{"content_type": "image/jpeg", "size_bytes": 1}] * 11}, 422),
                         ({"images": [{"content_type": "image/heic", "size_bytes": 10}]}, 422),
                         ({"images": [{"content_type": "image/jpeg", "size_bytes": 10 * 1024 * 1024 + 1}]}, 413)):
        seller.call("POST", "/api/v1/videos/photo-uploads", body=body, expect=status)

    left = photo_post(seller, [("image/png", PNG)])
    seller.call("DELETE", "/api/v1/videos/uploads/{id}", f"/api/v1/videos/uploads/{left['video_id']}", expect=204)

    # Until step 2c the media-worker dead-letters photo posts: never failed, never ready.
    time.sleep(3)
    check(seller.call(*get, headers=PHOTOS)["status"] == "processing", "the worker touched a photo post")
    seller.call("DELETE", "/api/v1/videos/{id}", f"/api/v1/videos/{pid}", expect=204)


def videos(seller):
    video_id = upload_video(seller, f"Danse e2e {RUN}", f"Parcours complet #e2e{RUN}")

    def ready():
        v = seller.call("GET", "/api/v1/videos/{id}", f"/api/v1/videos/{video_id}")
        check(v["status"] != "failed", f"video processing failed: {v}")
        return v if v["status"] == "ready" else None
    video = wait_for("video ready", ready, timeout=180, interval=3)
    check(video.get("hls_url") and video.get("thumbnail_url"), "ready video without media URLs")
    status, playlist, _ = http("GET", video["hls_url"])
    check(status == 200 and b"#EXTM3U" in playlist, f"HLS playlist not served: {status}")
    mine = seller.call("GET", "/api/v1/videos", query={"limit": 10})
    check(any(v["id"] == video_id for v in mine["items"]), "my videos does not list the new video")

    # An upload abandoned before completion.
    init = seller.call("POST", "/api/v1/videos/uploads", expect=(200, 201), body={
        "title": "Abandon", "content_type": "video/mp4", "size_bytes": 1024})
    seller.call("DELETE", "/api/v1/videos/uploads/{id}", f"/api/v1/videos/uploads/{init['video_id']}", expect=(200, 204))
    return video_id


def social(seller, buyer, video_id):
    uid = seller.user["id"]
    buyer.call("PUT", "/api/v1/users/{userId}/follow", f"/api/v1/users/{uid}/follow", expect=(200, 204))
    buyer.call("PUT", "/api/v1/videos/{videoId}/like", f"/api/v1/videos/{video_id}/like", expect=(200, 204))
    comment = buyer.call("POST", "/api/v1/videos/{videoId}/comments", f"/api/v1/videos/{video_id}/comments",
                         body={"content": "Superbe !"}, expect=(200, 201))
    reply = seller.call("POST", "/api/v1/videos/{videoId}/comments", f"/api/v1/videos/{video_id}/comments",
                        body={"content": "Merci", "parent_id": comment["id"]}, expect=(200, 201))
    listed = buyer.call("GET", "/api/v1/videos/{videoId}/comments", f"/api/v1/videos/{video_id}/comments")
    check(any(c["id"] == comment["id"] for c in listed["items"]), "comment not listed")
    replies = buyer.call("GET", "/api/v1/comments/{commentId}/replies", f"/api/v1/comments/{comment['id']}/replies")
    check(any(c["id"] == reply["id"] for c in replies["items"]), "reply not listed")
    buyer.call("PATCH", "/api/v1/comments/{commentId}", f"/api/v1/comments/{comment['id']}", body={"content": "Superbe vidéo !"})
    buyer.call("PUT", "/api/v1/comments/{commentId}/like", f"/api/v1/comments/{reply['id']}/like", expect=(200, 204))
    buyer.call("DELETE", "/api/v1/comments/{commentId}/like", f"/api/v1/comments/{reply['id']}/like", expect=(200, 204))
    buyer.call("POST", "/api/v1/videos/{videoId}/shares", f"/api/v1/videos/{video_id}/shares", body={"channel": "whatsapp"}, expect=(200, 201, 204))

    def counted():
        s = buyer.call("GET", "/api/v1/videos/{videoId}/stats", f"/api/v1/videos/{video_id}/stats")
        return s if s["likes"] >= 1 and s["comments"] >= 1 else None
    stats = wait_for("video stats", counted, timeout=30)
    check(stats.get("liked_by_me") is True, f"liked_by_me false: {stats}")
    many = buyer.call("GET", "/api/v1/videos/stats", query={"ids": video_id})
    check(len(many["items"] if isinstance(many, dict) else many) == 1, "videos/stats did not return the video")
    user_stats = buyer.call("GET", "/api/v1/users/{userId}/stats", f"/api/v1/users/{uid}/stats")
    check(user_stats["followers"] >= 1, f"followers not counted: {user_stats}")
    return comment, reply


def feeds(buyer, video_id):
    limits = iter(range(30, 0, -1))

    def in_following():
        # Page 1 is cached 60 s per viewer and limit: each probe asks another limit.
        page = buyer.call("GET", "/api/v1/feed/following", query={"limit": next(limits)})
        return any(i["video_id"] == video_id for i in page["items"])
    wait_for("video in Abonnements", in_following, timeout=40)
    buyer.call("GET", "/api/v1/feed/for-you", query={"limit": 10})
    buyer.call("GET", "/api/v1/feed/trending", query={"limit": 10})


def search(buyer, seller_username, video_title):
    buyer.call("GET", "/api/v1/search/suggest", query={"q": seller_username[:4]})
    wait_for("user indexed", lambda: any(u.get("username") == seller_username for u in
             buyer.call("GET", "/api/v1/search?type=users", query={"q": seller_username})["items"]), timeout=60)
    wait_for("video indexed", lambda: buyer.call("GET", "/api/v1/search?type=videos", query={"q": video_title})["items"], timeout=60)
    buyer.call("GET", "/api/v1/search?type=hashtags", query={"q": f"e2e{RUN}"})


def notifications(seller):
    page = wait_for("notifications of the seller", lambda: seller.call("GET", "/api/v1/notifications")["items"], timeout=60)
    count = seller.call("GET", "/api/v1/notifications/unread-count")
    check(any(isinstance(v, int) and v >= 1 for v in count.values()), f"unread count: {count}")
    seller.call("PATCH", "/api/v1/notifications/{id}/read", f"/api/v1/notifications/{page[0]['id']}/read", expect=(200, 204))
    seller.call("POST", "/api/v1/notifications/read-all", expect=(200, 204))
    prefs = seller.call("GET", "/api/v1/notifications/preferences")
    seller.call("PATCH", "/api/v1/notifications/preferences", body={"likes": not prefs.get("likes", True)})


def report(buyer, video_id):
    buyer.call("POST", "/api/v1/moderation/reports", expect=(200, 201, 202),
               body={"target_type": "video", "target_id": video_id, "reason": "spam", "comment": "e2e"})


def shop(seller, buyer):
    handle = f"e2e{RUN}"
    seller.call("POST", "/api/v1/shops", expect=201,
                body={"name": f"Boutique E2E {RUN}", "handle": handle, "country_code": "CI", "description": "Pagnes et bijoux"})
    mine = seller.call("GET", "/api/v1/shops/me")
    check(mine["handle"] == handle, f"shops/me: {mine}")
    seller.call("PATCH", "/api/v1/shops/me", body={"description": "Pagnes, bijoux et sacs"})
    presigned_image(seller, "/api/v1/shops/me/logo/upload-url", None, "/api/v1/shops/me/logo", None)
    check(seller.call("GET", "/api/v1/shops/me").get("logo_url"), "logo not set after confirmation")
    removed = seller.call("DELETE", "/api/v1/shops/me/logo")
    check(removed.get("logo_url") is None, f"logo still set after removal: {removed.get('logo_url')}")

    product = seller.call("POST", "/api/v1/shops/me/products", expect=201, body={
        "title": f"Pagne wax {RUN}", "description": "Coton, 6 yards",
        "variants": [{"title": "Bleu", "price": 15000, "stock": 10}, {"title": "Rouge", "price": 16000, "stock": 5}]})
    pid = product["product_id"]
    images_template = "/api/v1/shops/me/products/{productId}/images"
    for _ in range(2):
        presigned_image(seller, images_template + "/upload-url", f"/api/v1/shops/me/products/{pid}/images/upload-url",
                        images_template, f"/api/v1/shops/me/products/{pid}/images")
    product = seller.call("PATCH", "/api/v1/shops/me/products/{productId}", f"/api/v1/shops/me/products/{pid}",
                          body={"status": "active"})
    check(product["status"] == "active", f"product not published: {product.get('status')}")
    variant = product["variants"][0]
    seller.call("PUT", "/api/v1/shops/me/products/{productId}/variants/{variantId}/stock",
                f"/api/v1/shops/me/products/{pid}/variants/{variant['variant_id']}/stock", body={"stock_on_hand": 12})
    seller.call("DELETE", "/api/v1/shops/me/products/{productId}/images/{imageId}",
                f"/api/v1/shops/me/products/{pid}/images/{product['images'][-1]['image_id']}", expect=(200, 204))
    listed = seller.call("GET", "/api/v1/shops/me/products")
    check(any(p["product_id"] == pid for p in listed["items"]), "my products does not list the product")
    seller.call("GET", "/api/v1/shops/me/products/{productId}", f"/api/v1/shops/me/products/{pid}")

    draft = seller.call("POST", "/api/v1/shops/me/products", expect=201,
                        body={"title": "Brouillon", "variants": [{"title": "Unique", "price": 1000, "stock": 1}]})
    seller.call("DELETE", "/api/v1/shops/me/products/{productId}", f"/api/v1/shops/me/products/{draft['product_id']}", expect=(200, 204))

    public = buyer.call("GET", "/api/v1/shops/{handle}", f"/api/v1/shops/{handle}")
    check(public["handle"] == handle, "public shop")
    page = buyer.call("GET", "/api/v1/shops/{handle}/products", f"/api/v1/shops/{handle}/products")
    check(any(p["product_id"] == pid for p in page["items"]), "public catalogue does not list the product")
    buyer.call("GET", "/api/v1/products/{productId}", f"/api/v1/products/{pid}")

    seller.call("POST", "/api/v1/shops/me/close", expect=(200, 204))
    status, _, _ = http("GET", BASE + f"/api/v1/shops/{handle}")
    check(status == 404, f"closed shop still public: {status}")
    seller.call("POST", "/api/v1/shops/me/reopen", expect=(200, 204))
    return handle, pid, variant["variant_id"]


def catalogue_search(buyer, handle, title):
    wait_for("product indexed", lambda: buyer.call("GET", "/api/v1/search?type=products", query={"q": title})["items"], timeout=60)
    wait_for("shop indexed", lambda: any(s.get("handle") == handle for s in
             buyer.call("GET", "/api/v1/search?type=shops", query={"q": handle})["items"]), timeout=60)


def place_order(buyer, variant_id, save_address=False):
    buyer.call("PUT", "/api/v1/cart/items/{variantId}", f"/api/v1/cart/items/{variant_id}", body={"quantity": 1})
    preview = buyer.call("POST", "/api/v1/checkout/preview", expect=201, body={
        "delivery": {"full_name": "Awa Koné", "phone": "+2250701020304", "city": "Abidjan",
                     "address": "Cocody, rue des Jardins", "landmark": "Face à la pharmacie"},
        "payment_method": "cash_on_delivery", "save_address": save_address,
        "address_label": "Maison" if save_address else None})
    placed = buyer.call("POST", "/api/v1/checkout/confirm", expect=201,
                        body={"preview_id": preview["preview_id"], "confirmed": True})
    check(placed["address_saved"] is save_address, f"address_saved: {placed['address_saved']}")
    order_id = placed["orders"][0]["order_id"]

    # Shop reserves the stock on poro.order.order.created; the order is then confirmed.
    def confirmed():
        o = buyer.call("GET", "/api/v1/orders/{orderId}", f"/api/v1/orders/{order_id}")
        check(o["status"] != "cancelled", f"order cancelled before confirmation: {o}")
        return o["status"] == "confirmed"
    wait_for("order confirmed by the stock reservation", confirmed, timeout=60)
    return order_id


def orders(seller, buyer, variant_id):
    # The order service learns the catalogue from shop events.
    def in_cart():
        status, raw, _ = http("PUT", BASE + f"/api/v1/cart/items/{variant_id}", {"quantity": 2},
                              {"Authorization": f"Bearer {buyer.token}"})
        return status == 200
    wait_for("product known to the cart", in_cart, timeout=60)
    cart = buyer.call("GET", "/api/v1/cart")
    check(cart["items"] if "items" in cart else cart.get("shops"), f"empty cart: {cart}")
    buyer.call("DELETE", "/api/v1/cart/items/{variantId}", f"/api/v1/cart/items/{variant_id}", expect=(200, 204))
    buyer.call("GET", "/api/v1/checkout/prefill")

    shipped = place_order(buyer, variant_id, save_address=True)
    seller_orders = wait_for("order seen by the seller", lambda: seller.call("GET", "/api/v1/seller/orders")["items"], timeout=30)
    check(any(o["order_id"] == shipped for o in seller_orders), "seller does not see the order")
    seller.call("GET", "/api/v1/seller/orders/{orderId}", f"/api/v1/seller/orders/{shipped}")
    seller.call("POST", "/api/v1/seller/orders/{orderId}/ship", f"/api/v1/seller/orders/{shipped}/ship",
                body={"tracking": "E2E-001"}, expect=(200, 204))
    buyer.call("POST", "/api/v1/orders/{orderId}/confirm-receipt", f"/api/v1/orders/{shipped}/confirm-receipt", expect=(200, 204))

    cancelled = place_order(buyer, variant_id)
    buyer.call("POST", "/api/v1/orders/{orderId}/cancel", f"/api/v1/orders/{cancelled}/cancel", expect=(200, 204))
    refused = place_order(buyer, variant_id)
    seller.call("POST", "/api/v1/seller/orders/{orderId}/cancel", f"/api/v1/seller/orders/{refused}/cancel", expect=(200, 204))

    mine = buyer.call("GET", "/api/v1/orders")
    check(len(mine["items"]) >= 3, f"{len(mine['items'])} orders, expected 3")
    detail = buyer.call("GET", "/api/v1/orders/{orderId}", f"/api/v1/orders/{shipped}")
    check(detail["status"] == "completed", f"order status after receipt: {detail['status']}")
    addresses = buyer.call("GET", "/api/v1/addresses")
    items = addresses["items"] if isinstance(addresses, dict) else addresses
    check(items, "address not saved at checkout")

    # The address book of Settings: add, edit, choose the default, delete.
    office = buyer.call("POST", "/api/v1/addresses", expect=201, body={
        "full_name": "Awa Koné", "phone": "+2250701020304", "city": "Abidjan",
        "address": "Plateau, avenue Chardy", "label": "Bureau", "is_default": False})
    check(office["is_default"] is False, "a second address must not become the default")
    office = buyer.call("PUT", "/api/v1/addresses/{addressId}", f"/api/v1/addresses/{office['address_id']}", body={
        "full_name": "Awa Koné", "phone": "+2250701020304", "city": "Abidjan",
        "address": "Plateau, avenue Chardy, 3e étage", "label": "Bureau"})
    check("3e étage" in office["address"], f"address not replaced: {office}")
    buyer.call("POST", "/api/v1/addresses/{addressId}/default", f"/api/v1/addresses/{office['address_id']}/default")
    book = buyer.call("GET", "/api/v1/addresses")
    book = book["items"] if isinstance(book, dict) else book
    check(book[0]["address_id"] == office["address_id"] and book[0]["is_default"], "the new default is not listed first")
    for a in book:
        buyer.call("DELETE", "/api/v1/addresses/{addressId}", f"/api/v1/addresses/{a['address_id']}", expect=(200, 204))

    # Emptying the cart from the cart screen.
    buyer.call("PUT", "/api/v1/cart/items/{variantId}", f"/api/v1/cart/items/{variant_id}", body={"quantity": 1})
    emptied = buyer.call("DELETE", "/api/v1/cart")
    check(not emptied.get("shops") and not emptied.get("unavailable"), f"cart not emptied: {emptied}")


def cleanup(seller, buyer, video_id, comment, reply):
    """What the app lets users undo: likes, comments, follows, a video."""
    buyer.call("DELETE", "/api/v1/videos/{videoId}/like", f"/api/v1/videos/{video_id}/like", expect=(200, 204))
    seller.call("DELETE", "/api/v1/comments/{commentId}", f"/api/v1/comments/{reply['id']}", expect=(200, 204))
    buyer.call("DELETE", "/api/v1/comments/{commentId}", f"/api/v1/comments/{comment['id']}", expect=(200, 204))
    buyer.call("DELETE", "/api/v1/users/{userId}/follow", f"/api/v1/users/{seller.user['id']}/follow", expect=(200, 204))
    seller.call("DELETE", "/api/v1/videos/{id}", f"/api/v1/videos/{video_id}", expect=(200, 204))
    status, _, _ = http("GET", BASE + f"/api/v1/videos/{video_id}", headers={"Authorization": f"Bearer {buyer.token}"})
    check(status == 404, f"deleted video still served: {status}")


def main():
    seller, buyer = Client("seller"), Client("buyer")

    def step(name, fn, *args):
        started = time.time()
        result = fn(*args)
        print(f"ok    {name} ({time.time() - started:.1f}s)", flush=True)
        return result

    suffix = int(RUN, 16) % 10_000_000
    step("guest feed", guest)
    step("seller signs in (SMS)", otp_login, seller, f"+22507{suffix:08d}")
    step("buyer signs in (SMS)", otp_login, buyer, f"+22505{suffix:08d}")
    step("account, refresh", account, seller)
    carol = step("email account and sessions", email_account)
    seller_name = f"e2e_s_{RUN}"
    step("seller profile", profile, seller, seller_name)
    step("buyer profile", profile, buyer, f"e2e_b_{RUN}")
    step("avatar upload", avatar, seller)
    seller.call("POST", "/api/v1/users/me/creator", expect=(200, 201))
    video_id = step("video upload and processing", videos, seller)
    if os.environ.get("E2E_PHOTO_POSTS") == "1":
        step("photo posts (step 2b, enabled)", photo_posts, seller)
    else:
        step("photo posts closed by default", photo_posts_closed, seller)
    comment, reply = step("likes, comments, follows", social, seller, buyer, video_id)
    step("feeds", feeds, buyer, video_id)
    step("search", search, buyer, seller_name, f"Danse e2e {RUN}")
    step("notifications", notifications, seller)
    step("report", report, buyer, video_id)
    handle, _, variant_id = step("shop, products, photos", shop, seller, buyer)
    step("catalogue search", catalogue_search, buyer, handle, f"Pagne wax {RUN}")
    step("cart, checkout, orders", orders, seller, buyer, variant_id)
    step("undo: likes, comments, follow, video", cleanup, seller, buyer, video_id, comment, reply)
    step("password reset, account deletion, logout", goodbye, carol)
    return seller, buyer, video_id


def coverage():
    app = set()
    for path in (ROOT / "apps/mobile-android/app/src/main/java").rglob("*Api.kt"):
        for verb, arg in re.findall(r"@(GET|POST|PUT|PATCH|DELETE|HTTP)\(([^)]*)\)", path.read_text()):
            if verb == "HTTP":
                verb = re.search(r'method\s*=\s*"(\w+)"', arg).group(1)
                route = re.search(r'path\s*=\s*"([^"]+)"', arg).group(1)
            else:
                route = re.search(r'"([^"]+)"', arg).group(1)
            app.add((verb, "/" + route))

    def norm(route):
        return re.sub(r"\{[^}]+\}", "{}", route)
    done = {(m, norm(t)) for m, t in covered}
    hit = sorted(r for r in app if (r[0], norm(r[1])) in done)
    missed = sorted(app - set(hit))
    share = len(hit) / len(app)
    print(f"\ncoverage: {len(hit)}/{len(app)} routes of the app ({share:.0%})")
    for verb, route in missed:
        print(f"  not exercised: {verb} {route}")
    return share


if __name__ == "__main__":
    try:
        main()
    except Failure as err:
        print(f"FAIL  {err}")
        coverage()
        sys.exit(1)
    share = coverage()
    if share < MIN_COVERAGE:
        print(f"FAIL  coverage {share:.0%} under {MIN_COVERAGE:.0%}")
        sys.exit(1)
