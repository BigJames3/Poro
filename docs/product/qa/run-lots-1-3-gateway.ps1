# QA lots 1-3 against the live local gateway (127.0.0.1:8000).
# No secrets in output. Device / TalkBack / airplane cases are out of scope.
$ErrorActionPreference = 'Stop'
$base = 'http://127.0.0.1:8000'
$results = [System.Collections.Generic.List[string]]::new()
$stamp = Get-Date -Format 'yyyyMMddHHmmss'

function Record([string]$id, [bool]$ok, [string]$detail) {
    $mark = if ($ok) { 'PASS' } else { 'FAIL' }
    $line = "$mark $id $detail"
    $script:results.Add($line)
    Write-Host $line
}

function Get-ErrorCode($json) {
    if ($null -eq $json) { return '' }
    $err = $json | Select-Object -ExpandProperty error -ErrorAction SilentlyContinue
    if ($null -eq $err) { return '' }
    return [string]($err | Select-Object -ExpandProperty code -ErrorAction SilentlyContinue)
}

function Invoke-Api {
    param(
        [string]$Method,
        [string]$Path,
        [hashtable]$Headers = @{},
        $Body = $null
    )
    $out = Join-Path $env:TEMP ("poro-qa-" + [guid]::NewGuid().ToString() + '.json')
    $args = @('-sS', '-o', $out, '-w', '%{http_code}', '-X', $Method, '-H', 'Content-Type: application/json')
    if ($Headers.Authorization) { $args += @('-H', "Authorization: $($Headers.Authorization)") }
    if ($null -ne $Body) {
        $payload = $Body | ConvertTo-Json -Compress -Depth 8
        $bodyFile = Join-Path $env:TEMP ("poro-qa-body-" + [guid]::NewGuid().ToString() + '.json')
        [IO.File]::WriteAllText($bodyFile, $payload)
        $args += @('--data-binary', "@$bodyFile")
    }
    $args += "$base$Path"
    $code = & curl.exe @args
    $raw = ''
    if (Test-Path $out) { $raw = [IO.File]::ReadAllText($out) }
    $json = $null
    if ($raw) { try { $json = $raw | ConvertFrom-Json } catch { } }
    return @{ Status = [int]$code; Json = $json; Raw = $raw; Code = (Get-ErrorCode $json) }
}

function Register-User([string]$role) {
    $email = "qa.lots13.$role.$stamp@poro.test"
    $res = Invoke-Api POST '/api/v1/auth/email/register' @{} @{
        email       = $email
        password    = 'QaPoro-13-ok'
        full_name   = "QA $role"
        device_name = 'QA-gateway'
    }
    if ($res.Status -ne 201) { throw "register $role failed $($res.Status) $($res.Raw)" }
    return @{
        Email  = $email
        Id     = $res.Json.data.user.id
        Token  = $res.Json.data.access_token
        Header = @{ Authorization = "Bearer $($res.Json.data.access_token)" }
    }
}

function New-Jpeg([string]$path) {
    # Minimal valid 1x1 JPEG.
    $hex = 'FFD8FFE000104A46494600010101004800480000FFDB004300080606070605080707070909080A0C140D0C0B0B0C1912130F141D1A1F1E1D1A1C1C20242E2720222C231C1C2837292C30313434341F27393D38323C2E333432FFC0000B080001000101011100FFC40014000100000000000000000000000000000000FFDA0008010100003F00D2CF20FFD9'
    $bytes = [byte[]]::new($hex.Length / 2)
    for ($i = 0; $i -lt $bytes.Length; $i++) {
        $bytes[$i] = [Convert]::ToByte($hex.Substring($i * 2, 2), 16)
    }
    [IO.File]::WriteAllBytes($path, $bytes)
}

# --- Auth + shop -----------------------------------------------------------
$seller = Register-User 'seller'
$buyer = Register-User 'buyer'
Record 'QA-AUTH-01' ($seller.Token -and $buyer.Token) 'two email accounts via gateway'

$handle = "qa$stamp"
$shop = Invoke-Api POST '/api/v1/shops' $seller.Header @{
    name         = 'Boutique QA'
    handle       = $handle
    country_code = 'CI'
}
Record 'AC-MX-shop-open' ($shop.Status -eq 201) "status=$($shop.Status)"

$unknown = Invoke-Api GET '/api/v1/shops/inconnu_qa_zzz'
Record 'AC-MX-02' ($unknown.Status -eq 404 -and $unknown.Code -eq 'shop_not_found') "status=$($unknown.Status) code=$($unknown.Code)"

$product = Invoke-Api POST '/api/v1/shops/me/products' $seller.Header @{
    title    = 'Pagne QA'
    variants = @(@{ title = 'Unique'; price = 15000; stock = 20 })
}
$productId = $product.Json.data.product_id
$variantId = $product.Json.data.variants[0].variant_id
Record 'QA-MX-draft' ($product.Status -eq 201 -and $productId) "status=$($product.Status)"

$publishBare = Invoke-Api PATCH "/api/v1/shops/me/products/$productId" $seller.Header @{ status = 'active' }
Record 'AC-MX-16' ($publishBare.Status -eq 422 -and $publishBare.Code -eq 'image_required') "status=$($publishBare.Status) code=$($publishBare.Code)"

$jpeg = Join-Path $env:TEMP "poro-qa-$stamp.jpg"
New-Jpeg $jpeg
$photoCount = 0
$photoOk = $true
$eleventhCode = ''
for ($n = 1; $n -le 11; $n++) {
    $up = Invoke-Api POST "/api/v1/shops/me/products/$productId/images/upload-url" $seller.Header @{ content_type = 'image/jpeg' }
    if ($n -eq 11) {
        $eleventhCode = $up.Code
        Record 'AC-MX-17b' ($up.Status -eq 422 -and $eleventhCode -eq 'too_many_images') "11th upload-url status=$($up.Status) code=$eleventhCode"
        break
    }
    if ($up.Status -ne 201) { $photoOk = $false; break }
    $url = $up.Json.data.upload_url
    $fields = $up.Json.data.fields
    $key = $up.Json.data.upload_key
    $form = @()
    if ($fields) {
        $fields.PSObject.Properties | ForEach-Object { $form += "-F"; $form += "$($_.Name)=$($_.Value)" }
    }
    $form += '-F'; $form += "file=@$jpeg"
    $curlArgs = @('-sS', '-o', 'NUL', '-w', '%{http_code}', '-X', 'POST') + $form + @($url)
    $posted = & curl.exe @curlArgs
    if ($posted -notin @('200', '201', '204')) { $photoOk = $false; break }
    $confirm = Invoke-Api PUT "/api/v1/shops/me/products/$productId/images" $seller.Header @{ upload_key = $key }
    if ($confirm.Status -ne 200) { $photoOk = $false; break }
    $photoCount++
}
Record 'AC-MX-17' ($photoOk -and $photoCount -eq 10) "confirmed=$photoCount"

$publish = Invoke-Api PATCH "/api/v1/shops/me/products/$productId" $seller.Header @{ status = 'active' }
Record 'AC-MX-01a' ($publish.Status -eq 200 -and $publish.Json.data.status -eq 'active') "status=$($publish.Status) product=$($publish.Json.data.status)"

$seen = $false
for ($i = 0; $i -lt 12; $i++) {
    Start-Sleep -Seconds 2
    $pub = Invoke-Api GET "/api/v1/shops/$handle"
    if ($pub.Status -eq 200) { $seen = $true; break }
}
Record 'AC-MX-01' $seen "public handle after $($i*2)s"

# --- Cart / checkout -------------------------------------------------------
function Cart-Qty($qty) {
    return Invoke-Api PUT "/api/v1/cart/items/$variantId" $buyer.Header @{ quantity = $qty }
}

$cartReady = $null
$readyQty = 0
for ($i = 0; $i -lt 15; $i++) {
    $cartReady = Cart-Qty 1
    $readyQty = 0
    if ($cartReady.Json.data.shops) {
        $readyQty = @($cartReady.Json.data.shops | ForEach-Object { $_.items } | ForEach-Object { $_.quantity } | Measure-Object -Sum).Sum
    }
    if ($cartReady.Status -eq 200 -and $readyQty -ge 1) { break }
    Start-Sleep -Seconds 2
}
Record 'QA-MX-catalog' ($cartReady.Status -eq 200 -and $readyQty -ge 1) "status=$($cartReady.Status) qty=$readyQty code=$($cartReady.Code)"

$add1 = Cart-Qty 1
$add2 = Cart-Qty 2
$qty = 0
if ($add2.Json.data.shops) {
    $qty = @($add2.Json.data.shops | ForEach-Object { $_.items } | ForEach-Object { $_.quantity } | Measure-Object -Sum).Sum
}
Record 'AC-MX-03' ($add2.Status -eq 200 -and $qty -eq 2) "absolute PUT 1 then 2 => qty=$qty"

$cap = Cart-Qty 99
$over = Cart-Qty 100
$qty99 = 0
if ($cap.Json.data.shops) {
    $qty99 = @($cap.Json.data.shops | ForEach-Object { $_.items } | ForEach-Object { $_.quantity } | Measure-Object -Sum).Sum
}
Record 'AC-MX-04' ($cap.Status -eq 200 -and $qty99 -eq 99 -and $over.Status -ge 400) "qty=$qty99 over=$($over.Status)"

$sellerCart = Invoke-Api PUT "/api/v1/cart/items/$variantId" $seller.Header @{ quantity = 1 }
$own = Invoke-Api POST '/api/v1/checkout/preview' $seller.Header @{
    delivery       = @{ full_name = 'Vendeur QA'; phone = '+2250700000001'; city = 'Abidjan'; address = 'Cocody' }
    payment_method = 'cash_on_delivery'
}
Record 'AC-MX-06' ($own.Code -eq 'own_shop') "cart=$($sellerCart.Status) status=$($own.Status) code=$($own.Code)"

$before = Invoke-Api GET '/api/v1/orders' $buyer.Header
$beforeCount = @($before.Json.data.items).Count

$preview = Invoke-Api POST '/api/v1/checkout/preview' $buyer.Header @{
    delivery       = @{ full_name = 'Acheteur QA'; phone = '+2250700000002'; city = 'Abidjan'; address = 'Plateau' }
    payment_method = 'cash_on_delivery'
}
$previewId = $preview.Json.data.preview_id
$afterPreview = Invoke-Api GET '/api/v1/orders' $buyer.Header
$afterPreviewCount = @($afterPreview.Json.data.items).Count
Record 'AC-MX-07' ($preview.Status -eq 201 -and $previewId -and $afterPreviewCount -eq $beforeCount) "preview=$($preview.Status) orders=$afterPreviewCount"

$confirm = Invoke-Api POST '/api/v1/checkout/confirm' $buyer.Header @{ preview_id = $previewId; confirmed = $true }
$order = $null
if ($confirm.Json.data.orders) { $order = @($confirm.Json.data.orders)[0] }
$orderId = $order.order_id
$orderStatus = $order.status
$paid = $false
if ($order) { $paid = [bool]$order.paid }
Record 'AC-MX-08' ($confirm.Status -eq 201 -and $orderStatus -in @('confirmed', 'pending') -and -not $paid) "http=$($confirm.Status) status=$orderStatus paid=$paid code=$($confirm.Code)"

$confirm2 = Invoke-Api POST '/api/v1/checkout/confirm' $buyer.Header @{ preview_id = $previewId; confirmed = $true }
$order2 = $null
if ($confirm2.Json.data.orders) { $order2 = @($confirm2.Json.data.orders)[0].order_id }
Record 'AC-MX-09' ($order2 -and $orderId -and $order2 -eq $orderId) "first=$orderId second=$order2"

# New preview then dirty the cart before confirm.
$null = Cart-Qty 1
$preview2 = Invoke-Api POST '/api/v1/checkout/preview' $buyer.Header @{
    delivery       = @{ full_name = 'Acheteur QA'; phone = '+2250700000002'; city = 'Abidjan'; address = 'Plateau' }
    payment_method = 'cash_on_delivery'
}
if ($preview2.Status -eq 201) {
    $null = Cart-Qty 2
    $stale = Invoke-Api POST '/api/v1/checkout/confirm' $buyer.Header @{ preview_id = $preview2.Json.data.preview_id; confirmed = $true }
    Record 'AC-MX-10' ($stale.Code -eq 'preview_outdated') "status=$($stale.Status) code=$($stale.Code)"
} else {
    Record 'AC-MX-10' $false "second preview status=$($preview2.Status) $($preview2.Raw)"
}

$prefill = Invoke-Api GET '/api/v1/checkout/prefill' $buyer.Header
Record 'AC-MX-prefill' ($prefill.Status -eq 200) "source=$($prefill.Json.data.source) lookup=$($prefill.Json.data.account_lookup)"

# --- Moderation ------------------------------------------------------------
$target = [guid]::NewGuid().ToString()
$r1 = Invoke-Api POST '/api/v1/moderation/reports' $buyer.Header @{
    target_type = 'video'
    target_id   = $target
    reason      = 'spam'
}
Record 'AC-MOD-01' ($r1.Status -eq 201 -and $r1.Json.data.created -eq $true) "status=$($r1.Status) created=$($r1.Json.data.created)"

$r2 = Invoke-Api POST '/api/v1/moderation/reports' $buyer.Header @{
    target_type = 'video'
    target_id   = $target
    reason      = 'spam'
}
Record 'AC-MOD-02' ($r2.Status -eq 200 -and $r2.Json.data.created -eq $false) "status=$($r2.Status) created=$($r2.Json.data.created)"

$self = Invoke-Api POST '/api/v1/moderation/reports' $seller.Header @{
    target_type = 'user'
    target_id   = $seller.Id
    reason      = 'spam'
}
Record 'AC-MOD-03' ($self.Status -eq 422 -and $self.Code -eq 'cannot_report_self') "status=$($self.Status) code=$($self.Code)"

$badReason = Invoke-Api POST '/api/v1/moderation/reports' $buyer.Header @{
    target_type = 'video'
    target_id   = [guid]::NewGuid().ToString()
    reason      = 'not_a_reason'
}
Record 'AC-MOD-04' ($badReason.Status -ge 400) "status=$($badReason.Status)"

$long = Invoke-Api POST '/api/v1/moderation/reports' $buyer.Header @{
    target_type = 'comment'
    target_id   = [guid]::NewGuid().ToString()
    reason      = 'harassment'
    comment     = ('x' * 501)
}
Record 'AC-MOD-05' ($long.Status -ge 400) "status=$($long.Status)"

$comment = Invoke-Api POST '/api/v1/moderation/reports' $buyer.Header @{
    target_type = 'comment'
    target_id   = [guid]::NewGuid().ToString()
    reason      = 'harassment'
}
Record 'AC-MOD-06' ($comment.Status -eq 201) "status=$($comment.Status)"

$rateHit = $false
$rateStatus = 0
for ($n = 0; $n -lt 12; $n++) {
    $rr = Invoke-Api POST '/api/v1/moderation/reports' $buyer.Header @{
        target_type = 'video'
        target_id   = [guid]::NewGuid().ToString()
        reason      = 'spam'
    }
    if ($rr.Status -eq 429) { $rateHit = $true; $rateStatus = 429; break }
    $rateStatus = $rr.Status
}
Record 'AC-MOD-07' $rateHit "last_status=$rateStatus"

$cases = Invoke-Api GET '/api/v1/moderation/cases' $buyer.Header
Record 'AC-SEC-03' ($cases.Status -in 401, 403, 404) "member cases status=$($cases.Status)"

# --- Feed / search smoke (current contracts) --------------------------------
$trend = Invoke-Api GET '/api/v1/feed/trending'
$foryou = Invoke-Api GET '/api/v1/feed/for-you'
Record 'B-FEED-current' ($trend.Status -eq 200 -and $foryou.Status -eq 401) "trending=$($trend.Status) for-you=$($foryou.Status)"

$search = Invoke-Api GET '/api/v1/search?q=qa&type=videos'
Record 'SEARCH-smoke' ($search.Status -in 200, 401, 503) "status=$($search.Status) (503 if search container is down)"

Write-Host ''
Write-Host '=== SUMMARY ==='
$results | ForEach-Object { Write-Host $_ }
$fail = @($results | Where-Object { $_ -like 'FAIL *' }).Count
$pass = @($results | Where-Object { $_ -like 'PASS *' }).Count
Write-Host "passed=$pass failed=$fail"
if ($fail -gt 0) { exit 1 }
