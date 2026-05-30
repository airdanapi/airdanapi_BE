param (
    [string]$BaseUrl = "http://localhost:8080",
    [int]$DelayMs = 2000
)

# JWT Admin bypass token
$Token = "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJhdWQiOlsiZWNvc3lzdGVtIl0sImV4cCI6MTgxMTIyMTc3NywiaWF0IjoxNzc5Njg1Nzc3LCJpc3MiOiJzbWFydGJhbmsiLCJqdGkiOiJkZW1vLXRva2VuLWxpdmUtdjMiLCJuYmYiOjE3Nzk2ODU0NzAsInJvbGVzIjpbIm9wZXJhdG9yIl0sInNjb3BlcyI6WyJtYXJrZXRwbGFjZTpyZWFkIiwibWFya2V0cGxhY2U6d3JpdGUiLCJhZG1pbjpyZWFkIiwiYWRtaW46d3JpdGUiXSwic3ViIjoiYWRtaW4ifQ.dIr4g6MEA4iOAvaKeUfNhPMDWjODUhQ7vi4PM2TAvIH0MpiJnXOZhvVNX6raV-SYlilm-9pfZl8dFylRXL7xC7azHy5WcVrcE9mzxxxK9yKYdBZlPpTNG16xuF90VbEtpPW2xk-eb0ZQzv-3ciaUfFjlU1NQOzS_bMS-wtFVL4DIaX68b7Atkch4sHA7eLafgdmTpqxZM5INQMaJI97Y1Xmuvb_eGZAMqazHk9kRZLr5xm9CCRNgZUid5iRRYy-wbPk1jz1k8pFtgeKm_Mas2ES-faEg9bWAVnMyN4sOhhwwRHzXqYCTapEWc7Bdvf0N3ox1CLBWvZr7b8T_qotrrg"

Write-Host "Starting Demo Traffic Generator for Gateway Console..." -ForegroundColor Green
Write-Host "Target: $BaseUrl"
Write-Host "Delay: $($DelayMs)ms"
Write-Host "Press Ctrl+C to stop.`n"

$Counter = 1

while ($true) {
    try {
        # Randomize endpoint
        $r = Get-Random -Minimum 1 -Maximum 100
        
        $endpoint = "/health"
        $method = "GET"
        
        if ($r -lt 40) {
            $endpoint = "/ready"
        } elseif ($r -lt 70) {
            $endpoint = "/integrator/logging"
        } elseif ($r -lt 90) {
            $endpoint = "/integrator/validasi_request"
            $method = "POST"
        } else {
            $endpoint = "/api/v1/unknown/route"
        }
        
        $Uri = "$BaseUrl$endpoint"
        Write-Host "[$Counter] Requesting $method $Uri"
        
        if ($method -eq "GET") {
            Invoke-RestMethod -Uri $Uri -Method Get -Headers @{ "Authorization" = "Bearer $Token" } -ErrorAction SilentlyContinue | Out-Null
        } else {
            Invoke-RestMethod -Uri $Uri -Method Post -Headers @{ "Authorization" = "Bearer $Token" } -Body "{}" -ErrorAction SilentlyContinue | Out-Null
        }
        
        $Counter++
    } catch {
        # ignore errors for dummy traffic
    }
    
    Start-Sleep -Milliseconds $DelayMs
}
