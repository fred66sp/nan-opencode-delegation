# Instala la delegación de subagentes nan en la configuración global de OpenCode.
# Uso: powershell -ExecutionPolicy Bypass -File install.ps1
#Requires -Version 5.1
$ErrorActionPreference = 'Stop'
$here = Split-Path -Parent $MyInvocation.MyCommand.Path

$configRoot = Join-Path $HOME '.config\opencode'
if ($env:XDG_CONFIG_HOME) { $configRoot = Join-Path $env:XDG_CONFIG_HOME 'opencode' }

if (-not (Get-Command opencode -ErrorAction SilentlyContinue)) {
    Write-Host 'opencode no esta en el PATH. Instalalo primero: https://opencode.ai'
    exit 1
}

# 1. Los cinco agentes: verificacion, revision, documentacion, interfaz y orquestador.
$agentsDir = Join-Path $configRoot 'agents'
New-Item -ItemType Directory -Force -Path $agentsDir | Out-Null
Copy-Item -Force (Join-Path $here 'agents\*.md') $agentsDir
Write-Host "Agentes copiados en $agentsDir :"
Get-ChildItem $agentsDir -Filter *.md | ForEach-Object { Write-Host "  $($_.Name)" }

# 2. Modelos de los agentes integrados explore y general.
$dest = @('opencode.jsonc', 'opencode.json') |
    ForEach-Object { Join-Path $configRoot $_ } |
    Where-Object { Test-Path $_ } |
    Select-Object -First 1

if (-not $dest) {
    $dest = Join-Path $configRoot 'opencode.jsonc'
    Copy-Item (Join-Path $here 'config\opencode.jsonc') $dest
    Write-Host "Configuracion creada: $dest"
}
else {
    $backup = "$dest.bak.$(Get-Date -Format yyyyMMddHHmmss)"
    Copy-Item $dest $backup
    try {
        $data = Get-Content $dest -Raw | ConvertFrom-Json
        if (-not $data.PSObject.Properties['agent']) {
            $data | Add-Member -NotePropertyName agent -NotePropertyValue ([pscustomobject]@{})
        }
        $agent = $data.agent
        foreach ($pair in @(
            @{ Name = 'explore'; Model = 'nan/glm5.3-flash';   Variant = 'low'  },
            @{ Name = 'general'; Model = 'nan/deepseek-v4-flash'; Variant = 'high' }
        )) {
            $entry = [pscustomobject]@{ model = $pair.Model; variant = $pair.Variant }
            if ($agent.PSObject.Properties[$pair.Name]) { $agent.$($pair.Name) = $entry }
            else { $agent | Add-Member -NotePropertyName $pair.Name -NotePropertyValue $entry }
        }
        $utf8 = New-Object System.Text.UTF8Encoding($false)
        [System.IO.File]::WriteAllText($dest, (ConvertTo-Json $data -Depth 100), $utf8)
        Write-Host "Configuracion actualizada: $dest (copia previa en $backup)"
    }
    catch {
        Write-Host "No pude actualizar $dest automaticamente: el fichero tiene comentarios JSONC."
        Write-Host "Copia previa en $backup. Anade a mano este bloque dentro del objeto raiz:"
        Write-Host ''
        Write-Host '  "agent": {'
        Write-Host '    "explore": { "model": "nan/glm5.3-flash", "variant": "low" },'
        Write-Host '    "general": { "model": "nan/deepseek-v4-flash", "variant": "high" }'
        Write-Host '  },'
    }
}

Write-Host ''
Write-Host 'Siguiente paso, si el proveedor nan no esta autenticado en esta maquina:'
Write-Host '  opencode auth login        # elige el proveedor NaN y pega su API key'
Write-Host ''
Write-Host 'Comprobacion:'
Write-Host "  opencode models | Select-String 'nan/'"
Write-Host "  opencode api get /api/agent | Select-String 'orchestrator|verify|review|docs|ui'"
