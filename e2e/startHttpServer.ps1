$listener = New-Object System.Net.HttpListener;
$listener.Prefixes.Add("http://{{.Ip}}:{{.Port}}/");
$listener.Prefixes.Add("http://{{.Ip}}:{{nextPort .Port}}/");
$listener.Start();
Write-Host "Listening on http://{{.Ip}}:{{.Port}}/";
while ($listener.IsListening)
{
  $context = $listener.GetContext();
  $response = $context.Response;
  $content = [System.Text.Encoding]::UTF8.GetBytes("<html><body>Hello, Windows HTTP Server</body></html>");
  $response.ContentLength64 = $content.Length;
  $response.OutputStream.Write($content, 0, $content.Length);
  $response.Close();
}
