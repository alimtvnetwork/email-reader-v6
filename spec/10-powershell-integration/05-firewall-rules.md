# Windows Firewall Configuration

> **Version:** 1.1.0  
> **Updated:** 2026-02-04  
> **Status:** Active

---

## Overview

The PowerShell runner can automatically configure Windows Firewall inbound rules to allow network access to the Go backend server.

---

## Automatic Configuration

### Using the Script

```powershell
# Run as Administrator
.\run.ps1 -OpenFirewall
```

This creates inbound rules for all ports in `powershell.json`:

```json
{
  "ports": [8080, 8081]
}
```

### Created Rules

For each port, the script creates:

| Property | Value |
|----------|-------|
| DisplayName | `LLM Runner (Go Backend) TCP {port}` |
| Direction | Inbound |
| Action | Allow |
| Protocol | TCP |
| LocalPort | {port} |
| Profile | Private, Domain |

---

## Manual Configuration

If automatic configuration fails:

### PowerShell (Admin)

```powershell
# Create rule for port 8080
New-NetFirewallRule `
    -DisplayName "LLM Runner (Go Backend) TCP 8080" `
    -Direction Inbound `
    -Action Allow `
    -Protocol TCP `
    -LocalPort 8080 `
    -Profile Private,Domain

# Create rule for port 8081
New-NetFirewallRule `
    -DisplayName "LLM Runner (Go Backend) TCP 8081" `
    -Direction Inbound `
    -Action Allow `
    -Protocol TCP `
    -LocalPort 8081 `
    -Profile Private,Domain
```

### Windows Firewall GUI

1. Open **Windows Defender Firewall with Advanced Security**
2. Click **Inbound Rules** → **New Rule...**
3. Select **Port** → **Next**
4. Select **TCP**, enter port (e.g., `8080`) → **Next**
5. Select **Allow the connection** → **Next**
6. Check **Domain** and **Private** (uncheck Public) → **Next**
7. Name: `LLM Runner (Go Backend) TCP 8080` → **Finish**

---

## Verification

### Check Existing Rules

```powershell
# List all LLM Runner rules
Get-NetFirewallRule -DisplayName "LLM Runner*"

# Check specific port
Get-NetFirewallRule -DisplayName "LLM Runner (Go Backend) TCP 8080"
```

### Test Port Access

From another machine on the network:

```powershell
# Test connection
Test-NetConnection -ComputerName YOUR_IP -Port 8080
```

---

## Removing Rules

### Remove All LLM Runner Rules

```powershell
Get-NetFirewallRule -DisplayName "LLM Runner*" | Remove-NetFirewallRule
```

### Remove Specific Rule

```powershell
Remove-NetFirewallRule -DisplayName "LLM Runner (Go Backend) TCP 8080"
```

---

## Security Considerations

### Profile Selection

| Profile | When Active | Recommendation |
|---------|-------------|----------------|
| Domain | Corporate network with domain controller | ✅ Allow |
| Private | Home/trusted network | ✅ Allow |
| Public | Coffee shop, airport, etc. | ❌ Block |

The script only enables Domain and Private profiles by default.

### Port Selection

| Port | Common Usage | Risk Level |
|------|--------------|------------|
| 8080 | HTTP alternate | Low |
| 8081 | Additional service | Low |
| 80 | Standard HTTP | Medium (conflicts) |
| 443 | Standard HTTPS | Medium (conflicts) |
| 3000 | Dev servers | Low |

---

## Troubleshooting

### "Not Running as Administrator"

```
WARNING: -OpenFirewall requires Administrator. 
Re-run PowerShell as Admin to apply firewall rules.
TIP: Right-click PowerShell → Run as Administrator
```

**Solution:** Right-click PowerShell and select "Run as Administrator"

### "New-NetFirewallRule Not Available"

```
WARNING: New-NetFirewallRule not available. 
Skipping automatic firewall setup.
```

**Solutions:**
- Use PowerShell 5.1+ (not older versions)
- Use full PowerShell (not PowerShell ISE)
- Manually create rules via GUI

### Rule Exists But Port Blocked

```powershell
# Check if rule is enabled
Get-NetFirewallRule -DisplayName "LLM Runner*" | 
    Select-Object DisplayName, Enabled, Action

# Enable if disabled
Enable-NetFirewallRule -DisplayName "LLM Runner (Go Backend) TCP 8080"
```

### Multiple Rules Conflict

```powershell
# Remove all and recreate
Get-NetFirewallRule -DisplayName "LLM Runner*" | Remove-NetFirewallRule
.\run.ps1 -OpenFirewall
```

---

## Implementation Reference

```powershell
function Ensure-FirewallRules {
    param([int[]]$Ports = @(8080, 8081))

    # Check admin rights
    if (-not (Test-IsAdmin)) {
        Write-Host "WARNING: Requires Administrator" -ForegroundColor Yellow
        return
    }

    # Check cmdlet availability
    if (-not (Test-Command "New-NetFirewallRule")) {
        Write-Host "WARNING: Cmdlet not available" -ForegroundColor Yellow
        return
    }

    foreach ($p in $Ports) {
        $ruleName = "LLM Runner (Go Backend) TCP $p"
        $existing = Get-NetFirewallRule -DisplayName $ruleName -ErrorAction SilentlyContinue
        
        if ($null -eq $existing) {
            New-NetFirewallRule `
                -DisplayName $ruleName `
                -Direction Inbound `
                -Action Allow `
                -Protocol TCP `
                -LocalPort $p `
                -Profile Private,Domain `
                | Out-Null
            Write-Host "✓ Firewall rule added: $ruleName" -ForegroundColor Green
        } else {
            Write-Host "✓ Firewall rule exists: $ruleName" -ForegroundColor Green
        }
    }
}
```

---

## Cross-References

- [Script Reference](./02-script-reference.md) - Full function listing
- [Error Codes](./04-error-codes.md) - 9540-9542 firewall errors
