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

# 1. Los agentes: verificacion, revision, documentacion, interfaz y orquestador.
$agentsDir = Join-Path $configRoot 'agents'
New-Item -ItemType Directory -Force -Path $agentsDir | Out-Null
$stamp = Get-Date -Format yyyyMMddHHmmss
Get-ChildItem (Join-Path $here 'agents\*.md') | ForEach-Object {
    $target = Join-Path $agentsDir $_.Name
    if ((Test-Path $target) -and ((Get-FileHash $_.FullName).Hash -ne (Get-FileHash $target).Hash)) {
        Copy-Item $target ($target + '.bak.' + $stamp)
        Write-Host "Copia previa del agente: $target.bak.$stamp"
    }
    Copy-Item -Force $_.FullName $agentsDir
}
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
        $agentsProp = $data.PSObject.Properties['agents']
        if (-not $agentsProp -or $null -eq $agentsProp.Value) {
            if ($agentsProp) { $data.PSObject.Properties.Remove('agents') }
            $data | Add-Member -NotePropertyName agents -NotePropertyValue ([pscustomobject]@{})
        }
        elseif (-not ($agentsProp.Value -is [pscustomobject])) {
            throw "la clave 'agents' no es un objeto"
        }
        $agents = $data.agents
        $oldProp = $data.PSObject.Properties['agent']
        if ($oldProp -and ($oldProp.Value -is [pscustomobject])) {
            foreach ($name in @('explore', 'general')) {
                $entry = $oldProp.Value.PSObject.Properties[$name]
                if ($entry) {
                    if ($agents.PSObject.Properties[$name]) { $agents.($name) = $entry.Value }
                    else { $agents | Add-Member -NotePropertyName $name -NotePropertyValue $entry.Value }
                    $oldProp.Value.PSObject.Properties.Remove($name)
                }
            }
            if (@($oldProp.Value.PSObject.Properties).Count -eq 0) {
                $data.PSObject.Properties.Remove('agent')
            }
            else {
                Write-Host "Aviso: la configuracion conserva la clave obsoleta 'agent' con otras entradas; los overrides de explore y general se han movido a 'agents'."
            }
        }
        elseif ($oldProp) {
            Write-Host "Aviso: la clave obsoleta 'agent' no es un objeto; no se migra."
        }
        foreach ($pair in @(
            @{ Name = 'explore'; Model = [pscustomobject]@{ providerID = 'nan'; model = 'glm5.3-flash'; variant = 'low' } },
            @{ Name = 'general'; Model = [pscustomobject]@{ providerID = 'nan'; model = 'deepseek-v4-flash'; variant = 'high' } }
        )) {
            $entry = [pscustomobject]@{ model = $pair.Model }
            if ($agents.PSObject.Properties[$pair.Name]) { $agents.($pair.Name) = $entry }
            else { $agents | Add-Member -NotePropertyName $pair.Name -NotePropertyValue $entry }
        }
        $utf8 = New-Object System.Text.UTF8Encoding($false)
        [System.IO.File]::WriteAllText($dest, (ConvertTo-Json $data -Depth 100), $utf8)
        Write-Host "Configuracion actualizada: $dest (copia previa en $backup)"
    }
    catch {
        Write-Host "No pude actualizar $dest automaticamente: el fichero tiene comentarios JSONC o la clave 'agents' no es un objeto."
        Write-Host "Copia previa en $backup. Anade a mano este bloque dentro del objeto raiz:"
        Write-Host ''
        Write-Host '  "agents": {'
        Write-Host '    "explore": { "model": { "providerID": "nan", "model": "glm5.3-flash", "variant": "low" } },'
        Write-Host '    "general": { "model": { "providerID": "nan", "model": "deepseek-v4-flash", "variant": "high" } }'
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
