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

# 1. Los agentes —verificacion, revision, documentacion, interfaz, tablero y orquestador—
# y la herramienta del flujo, en el mismo directorio.
$agentsDir = Join-Path $configRoot 'agents'
New-Item -ItemType Directory -Force -Path $agentsDir | Out-Null
$stamp = Get-Date -Format yyyyMMddHHmmss
Get-ChildItem (Join-Path $here 'agents\*') -File | ForEach-Object {
    $target = Join-Path $agentsDir $_.Name
    if ((Test-Path $target) -and ((Get-FileHash $_.FullName).Hash -ne (Get-FileHash $target).Hash)) {
        Copy-Item $target ($target + '.bak.' + $stamp)
        Write-Host "Copia previa del agente: $target.bak.$stamp"
    }
    Copy-Item -Force $_.FullName $agentsDir
}
Write-Host "Agentes copiados en $agentsDir :"
Get-ChildItem $agentsDir -File | Where-Object { $_.Name -notlike '*.bak.*' } | ForEach-Object { Write-Host "  $($_.Name)" }

# 2. El plugin TUI del estado de orquestacion, con sus fuentes. Sin node_modules:
# OpenCode resuelve sus dependencias en runtime. Va en cli-plugins y se registra en
# cli.json: un paquete solo-TUI no puede vivir en plugins/ (el servidor lo cargaria
# y colisiona con su OpenTUI interno) y solo la TUI debe importarlo.
$pluginSrc = Join-Path $here 'plugins\orchestrator-status'
$pluginDir = Join-Path $configRoot 'cli-plugins\orchestrator-status'
Get-ChildItem $pluginSrc -Recurse -File | Where-Object { $_.FullName -notmatch '\\node_modules\\' } | ForEach-Object {
    $rel = $_.FullName.Substring($pluginSrc.Length).TrimStart('\')
    $target = Join-Path $pluginDir $rel
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $target) | Out-Null
    if ((Test-Path $target) -and ((Get-FileHash $_.FullName).Hash -ne (Get-FileHash $target).Hash)) {
        Copy-Item $target ($target + '.bak.' + $stamp)
        Write-Host "Copia previa del plugin: $target.bak.$stamp"
    }
    Copy-Item -Force $_.FullName $target
}
Write-Host "Plugin copiado en $pluginDir :"
Get-ChildItem $pluginDir -Recurse -File | Where-Object { $_.Name -notlike '*.bak.*' } | ForEach-Object { Write-Host "  $($_.Name)" }

$cliJson = Join-Path $configRoot 'cli.json'
$cliBak = Join-Path $configRoot ("cli.json.bak." + $stamp)
if (Test-Path $cliJson) { Copy-Item $cliJson $cliBak }
try {
    if (Test-Path $cliJson) { $cli = Get-Content $cliJson -Raw | ConvertFrom-Json } else { $cli = $null }
    if ($null -eq $cli) { $cli = [pscustomobject]@{} }
    if ($null -eq $cli.PSObject.Properties['plugins'] -or $null -eq $cli.plugins) {
        $cli | Add-Member -MemberType NoteProperty -Name 'plugins' -Value @()
    }
    if ($cli.plugins -isnot [array] -and $cli.plugins) {
        throw "la clave 'plugins' no es una lista"
    }
    $pluginsList = @($cli.plugins)
    if ($pluginsList -notcontains $pluginDir) {
        $cli.plugins = $pluginsList + $pluginDir
    }
    $cli | ConvertTo-Json -Depth 20 | Set-Content -Path $cliJson -Encoding UTF8
    Write-Host "Plugin registrado en cli.json (copia previa en $cliBak, si existia)"
} catch {
    if (Test-Path $cliBak) { Copy-Item $cliBak $cliJson -Force }
    Write-Host "No pude actualizar $cliJson automaticamente: $($_.Exception.Message)"
    Write-Host "Anade a mano la ruta del plugin a la clave plugins de cli.json:"
    Write-Host "  \"plugins\": [ \"$pluginDir\", ... ]"
}

# 3. Modelos de explore y general, y la regla global que deniega tasks_*.
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
            $entry = [pscustomobject]@{ model = $pair.Model; permissions = @([pscustomobject]@{ action = 'skill'; resource = '*'; effect = 'deny' }) }
            if ($agents.PSObject.Properties[$pair.Name]) { $agents.($pair.Name) = $entry }
            else { $agents | Add-Member -NotePropertyName $pair.Name -NotePropertyValue $entry }
        }
        $rule = [pscustomobject]@{ action = 'tasks_*'; resource = '*'; effect = 'deny' }
        $permsProp = $data.PSObject.Properties['permissions']
        if (-not $permsProp -or $null -eq $permsProp.Value) {
            if ($permsProp) { $data.PSObject.Properties.Remove('permissions') }
            $data | Add-Member -NotePropertyName permissions -NotePropertyValue @($rule)
        }
        elseif ($permsProp.Value -is [array]) {
            $hasRule = @($permsProp.Value | Where-Object { $_.action -eq 'tasks_*' -and $_.effect -eq 'deny' }).Count -gt 0
            if (-not $hasRule) { $data.permissions = @($permsProp.Value) + @($rule) }
        }
        elseif (($permsProp.Value -is [pscustomobject]) -and ($permsProp.Value.action -eq 'tasks_*') -and ($permsProp.Value.effect -eq 'deny')) {
            # ConvertFrom-Json desenrolla el array de un solo elemento: la regla ya esta.
        }
        elseif ($permsProp.Value -is [pscustomobject]) {
            $data.permissions = @($permsProp.Value) + @($rule)
        }
        else {
            Write-Host "Aviso: la clave 'permissions' no es una lista; no se anade la regla de tasks_*."
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
        Write-Host '    "explore": { "model": { "providerID": "nan", "model": "glm5.3-flash", "variant": "low" }, "permissions": [ { "action": "skill", "resource": "*", "effect": "deny" } ] },'
        Write-Host '    "general": { "model": { "providerID": "nan", "model": "deepseek-v4-flash", "variant": "high" }, "permissions": [ { "action": "skill", "resource": "*", "effect": "deny" } ] }'
        Write-Host '  },'
        Write-Host '  "permissions": ['
        Write-Host '    { "action": "tasks_*", "resource": "*", "effect": "deny" }'
        Write-Host '  ],'
    }
}

Write-Host ''
Write-Host 'Siguiente paso, si el proveedor nan no esta autenticado en esta maquina:'
Write-Host '  opencode auth login        # elige el proveedor NaN y pega su API key'
Write-Host ''
Write-Host 'Comprobacion:'
Write-Host "  opencode models | Select-String 'nan/'"
Write-Host "  opencode api get /api/agent | Select-String 'orchestrator|verify|review|docs|ui|tasks-agent'"
