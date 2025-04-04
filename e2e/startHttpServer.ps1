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
                $targetUrl = "http://" + $targetUrl
            };
            $c = New-Object System.Net.WebClient;
            $d = $c.DownloadData($targetUrl);
            $res.ContentLength64 = $d.Length;
            $res.OutputStream.Write($d, 0, $d.Length)
        } catch {
            $err = [System.Text.Encoding]::UTF8.GetBytes("Proxy error: $_");
            $res.StatusCode = 500;
            $res.ContentLength64 = $err.Length;
            $res.OutputStream.Write($err, 0, $err.Length)
        }
    } else {
        $c = [System.Text.Encoding]::UTF8.GetBytes("<html><body>Hello, Windows HTTP Server</body></html>");
        $res.ContentLength64 = $c.Length;
        $res.OutputStream.Write($c, 0, $c.Length)
    };
    $res.Close()
}
