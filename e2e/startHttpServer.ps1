Add-Type -AssemblyName System.Web;
$l = New-Object System.Net.HttpListener;
$l.Prefixes.Add("http://{{.Ip}}:{{.Port}}/");
$l.Prefixes.Add("http://{{.Ip}}:{{nextPort .Port}}/");
$l.Start();
Write-Host "Listening on http://{{.Ip}}:{{.Port}}/";
while ($l.IsListening) {
    $con = $l.GetContext();
    $req = $con.Request;
    $res = $con.Response;

    if ($req.Url.Query -match "url=([^&]+)") {
        try {
            $targetUrl = [System.Web.HttpUtility]::UrlDecode($matches[1]);
            if (-not $targetUrl.StartsWith("http")) {
                $targetUrl = "http://" + $targetUrl;
            }

            $userAgent = "Mozilla/5.0 ... Chrome";
            $response = Invoke-WebRequest -Uri $targetUrl -UserAgent $userAgent -UseBasicParsing;

            $contentBytes = [System.Text.Encoding]::UTF8.GetBytes($response.Content);

            if ($response.Headers["Content-Type"]) {
                $res.ContentType = $response.Headers["Content-Type"];
            }

            $res.ContentLength64 = $contentBytes.Length
            $res.OutputStream.Write($contentBytes, 0, $contentBytes.Length);
        } catch {
            $err = [System.Text.Encoding]::UTF8.GetBytes("Proxy error: $_");
            $res.StatusCode = 500;
            $res.OutputStream.Write($err, 0, $err.Length);
        }
    } else {
        $c = [System.Text.Encoding]::UTF8.GetBytes("<html><body>Hello, Windows HTTP Server</body></html>");
        $res.OutputStream.Write($c, 0, $c.Length);
    }
    $res.OutputStream.Flush()
    $res.Close()
}
