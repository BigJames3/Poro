# End-to-end smoke test of a running auth service (APP_ENV=dev, SMS_PROVIDER=log).
# The OTP code is read from the logs of the auth container.
#   .\scripts\smoke-test.ps1 -BaseUrl http://localhost:8081 -Container poro-auth
param(
    [string]$BaseUrl = "http://localhost:8081",
    [string]$Container = "poro-auth"
)
$ErrorActionPreference = "Stop"

Add-Type -AssemblyName System.Net.Http
$client = New-Object System.Net.Http.HttpClient

function Invoke-Api {
    param([string]$Method, [string]$Path, $Body = $null, [string]$Token = "")
    $request = New-Object System.Net.Http.HttpRequestMessage([System.Net.Http.HttpMethod]::new($Method), "$BaseUrl$Path")
    if ($Token) { $request.Headers.Add("Authorization", "Bearer $Token") }
    if ($null -ne $Body) {
        $request.Content = New-Object System.Net.Http.StringContent(($Body | ConvertTo-Json -Compress), [System.Text.Encoding]::UTF8, "application/json")
    }
    $response = $client.SendAsync($request).GetAwaiter().GetResult()
    $content = $response.Content.ReadAsStringAsync().GetAwaiter().GetResult()
    $headers = @{}
    foreach ($h in $response.Headers) { $headers[$h.Key] = ($h.Value -join ",") }
    $json = if ($content) { $content | ConvertFrom-Json } else { $null }
    return @{ Status = [int]$response.StatusCode; Body = $json; Headers = $headers }
}

function Assert-Equal($Expected, $Actual, [string]$Label) {
    if ($Expected -ne $Actual) { throw "FAIL $Label : expected '$Expected', got '$Actual'" }
    Write-Output "ok   $Label"
}

$phone = "+22507" + (Get-Random -Minimum 10000000 -Maximum 99999999)

$r = Invoke-Api GET "/health/ready"
Assert-Equal 200 $r.Status "readiness probe"

$r = Invoke-Api GET "/.well-known/jwks.json"
Assert-Equal "RS256" $r.Body.keys[0].alg "jwks publishes the RS256 key"

$r = Invoke-Api POST "/api/v1/auth/otp/request" @{ phone = $phone }
Assert-Equal 200 $r.Status "otp request"

$r = Invoke-Api POST "/api/v1/auth/otp/request" @{ phone = $phone }
Assert-Equal 429 $r.Status "second otp request hits the cooldown"
Assert-Equal "otp_throttled" $r.Body.error.code "throttle error code"
if (-not $r.Headers["Retry-After"]) { throw "FAIL Retry-After header missing" }
Write-Output "ok   Retry-After header present"

Start-Sleep -Milliseconds 500
$line = cmd /c "docker logs $Container 2>&1" | Select-String -Pattern "dev sms otp" | Where-Object { $_ -match [regex]::Escape($phone) } | Select-Object -Last 1
if (-not $line) { throw "FAIL no dev sms line for $phone in $Container logs" }
$code = [regex]::Match($line.ToString(), '"code":\s*"(\d{6})"').Groups[1].Value
if (-not $code) { throw "FAIL could not parse otp code" }
$wrong = if ($code -eq "000000") { "111111" } else { "000000" }

$r = Invoke-Api POST "/api/v1/auth/otp/verify" @{ phone = $phone; code = $wrong }
Assert-Equal "otp_invalid" $r.Body.error.code "wrong code is rejected"

$r = Invoke-Api POST "/api/v1/auth/otp/verify" @{ phone = $phone; code = $code }
Assert-Equal 200 $r.Status "right code signs in"
Assert-Equal "PERSONAL" ($r.Body.data.user.roles -join ",") "new account is PERSONAL"
$access1 = $r.Body.data.access_token
$refresh1 = $r.Body.data.refresh_token
if (-not $r.Body.meta.request_id) { throw "FAIL request_id missing from meta" }

$r = Invoke-Api POST "/api/v1/auth/otp/verify" @{ phone = $phone; code = $code }
Assert-Equal 401 $r.Status "a code is single use"

$r = Invoke-Api GET "/api/v1/auth/me" -Token $access1
Assert-Equal $phone $r.Body.data.phone "me returns the account"

$r = Invoke-Api POST "/api/v1/auth/refresh" @{ refresh_token = $refresh1 }
Assert-Equal 200 $r.Status "refresh rotates"
$refresh2 = $r.Body.data.refresh_token

$r = Invoke-Api POST "/api/v1/auth/refresh" @{ refresh_token = $refresh1 }
Assert-Equal 200 $r.Status "lost-response retry inside the grace window"
$refresh3 = $r.Body.data.refresh_token

$r = Invoke-Api POST "/api/v1/auth/refresh" @{ refresh_token = $refresh2 }
Assert-Equal "session_revoked" $r.Body.error.code "superseded token replay revokes the session"

$r = Invoke-Api POST "/api/v1/auth/refresh" @{ refresh_token = $refresh3 }
Assert-Equal 401 $r.Status "the whole session family is revoked"

$email = "smoke" + (Get-Random -Minimum 100000 -Maximum 999999) + "@Poro.Test"
$r = Invoke-Api POST "/api/v1/auth/email/register" @{ email = $email; password = "correct-horse-battery"; full_name = "Smoke Test" }
Assert-Equal 201 $r.Status "email register"
Assert-Equal $email.ToLower() $r.Body.data.user.email "email is normalized"

$r = Invoke-Api POST "/api/v1/auth/email/login" @{ email = $email.ToUpper(); password = "correct-horse-battery" }
Assert-Equal 200 $r.Status "email login is case insensitive"
$access = $r.Body.data.access_token
$refresh = $r.Body.data.refresh_token

$r = Invoke-Api POST "/api/v1/auth/email/login" @{ email = $email; password = "wrong-password-123" }
Assert-Equal "invalid_credentials" $r.Body.error.code "wrong password is rejected"

$r = Invoke-Api POST "/api/v1/auth/logout" -Token $access
Assert-Equal 204 $r.Status "logout"

$r = Invoke-Api GET "/api/v1/auth/me" -Token $access
Assert-Equal 401 $r.Status "access token is revoked after logout"

$r = Invoke-Api POST "/api/v1/auth/refresh" @{ refresh_token = $refresh }
Assert-Equal 401 $r.Status "refresh token is revoked after logout"

Write-Output "smoke test passed"
