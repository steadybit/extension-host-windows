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

      # Use Invoke-WebRequest instead of WebClient
      $userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36";
      $response = Invoke-WebRequest -Uri $targetUrl -UserAgent $userAgent -UseBasicParsing;

      # Handle the content based on the content type
      $contentBytes = [System.Text.Encoding]::UTF8.GetBytes($response.Content);
      if ($response.Headers.ContainsKey("Content-Type")) {
        $res.ContentType = $response.Headers["Content-Type"];
      }

      $res.OutputStream.Write($contentBytes, 0, $contentBytes.Length);
    } catch {
      $err = [System.Text.Encoding]::UTF8.GetBytes("Proxy error: $_");
      $res.StatusCode = 500;
      $res.OutputStream.Write($err, 0, $err.Length);
    }
  } else {
    $c = [System.Text.Encoding]::UTF8.GetBytes("<html><body>Hello, Windows HTTP Server</body></html>");
    $res.OutputStream.Write($c, 0, $c.Length);
  };
}
