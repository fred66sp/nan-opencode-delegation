// Package phase holds the eleven phases of the orchestrator flow as data and
// the transition semantics shared with the Python runner.
package phase

// Phase is one step of the flow. Field names mirror the `--json` contract that
// the plugins consume unchanged.
type Phase struct {
	Number   int      `json:"number"`
	Name     string   `json:"name"`
	When     string   `json:"when"`
	Evidence string   `json:"evidence"`
	Steps    []string `json:"steps"`
	Warnings []string `json:"warnings"`
}

// Evidence is one history entry: the phase that was left, its text and, for a
// skip, the phase jumped to.
type Evidence struct {
	Phase int    `json:"phase"`
	Text  string `json:"text"`
	Skip  int    `json:"skip,omitempty"`
}

// Count is the number of phases; the last one is terminal.
const Count = 11

// Phases is the flow in order. Numbers are assigned on load.
var Phases = []Phase{
	{
		Name:     "Inicio de sesión",
		When:     "al empezar cualquier sesión, antes de lanzar subagentes o tocar archivos",
		Evidence: "el `runner start` leído y el `AGENTS.md` del proyecto",
		Steps: []string{
			"Abrir la sesión con `runner start` desde la raíz del proyecto: memoria, tablero, Git, fase y encargos abiertos en una invocación. No invoques el arranque de memoria por separado: `start` ya lo hace.",
			"Leer el `AGENTS.md` del proyecto y su mapa de fuentes antes de decidir nada.",
			"Si la fase mostrada no es la 1 y empiezas trabajo nuevo en esta misma sesión, invoca `phase_reset` por `runner aif` con su motivo; si retomas el trabajo anterior, continúa desde la fase mostrada. El estado lo escribe solo el runner: el agente no puede tocarlo.",
		},
		Warnings: []string{
			"Nada se lanza antes de ejecutar `runner start` y leer el `AGENTS.md` del proyecto.",
			"El runner no comprueba el mundo real: informa antes de que falles; las guardas del tablero y del producto siguen siendo las que bloquean.",
		},
	},
	{
		Name:     "Clasificar el encargo",
		When:     "en cuanto llega la petición, antes de cualquier escritura",
		Evidence: "la clasificación del encargo y, si es planificación, el acuerdo explícito de la persona",
		Steps: []string{
			"Clasificar la petición: pregunta, revisión, plan o implementación.",
			"Pregunta, revisión o plan son solo lectura: no se escribe sin una petición explícita de implementar.",
			"Detectar planificación: ejemplos, matices sobre decisiones ya tomadas, «es un ejemplo», «ya lo abordaremos más adelante». Ahí no se abre tablero ni se lanzan subagentes hasta el acuerdo explícito.",
			"Separar hecho (comprobado en el repo o con una herramienta) de inferencia (razonada pero sin verificar) y de decisión (elección de diseño, del humano).",
		},
		Warnings: []string{
			"Sin tablero ni subagentes hasta el acuerdo explícito: una petición de plan no autoriza a implementar.",
			"Ante la duda entre plan e implementación, pregunta; no conviertas la ambigüedad en trabajo.",
		},
	},
	{
		Name:     "Tamaño y tablero",
		When:     "tras clasificar como implementación y antes de la primera escritura",
		Evidence: "la referencia de la tarea (`tk-n`), «trivial sin tarea, con autorización de la persona» o «tarea creada, sin épica»",
		Steps: []string{
			"Aplicar el test de la sesión interrumpida: si la sesión se parara ahora, ¿alguien podría terminar el trabajo con la petición original y `git diff`?",
			"Respuesta sí: si el cambio es muy trivial —uno o dos archivos como máximo, p. ej. una entrada en `.gitignore`—, no se crea tarea nueva, salvo que la persona la quiera; en otro caso, registra por AIF (`tasks-ai aif`) la tarea (`task` + `ODD` + tipo) —o una épica si el trabajo reviste importancia— con su criterio observable y la petición original en un comentario, y comprueba por CLI que quedó con su criterio antes de la primera edición.",
			"Respuesta no: trabajo grande. Registra por AIF, en una sola secuencia ordenada: una épica (`epic` + `ODD` + tipo), sus subtareas (`task` + `ODD` + tipo), el criterio observable de cada una y la petición original en un comentario. Continúa con tu proceso y, antes de la primera edición, comprueba por CLI que la épica quedó con su criterio.",
			"Comprobar el catálogo con `tasks-ai tags`: las etiquetas nacen ahí antes de usarse.",
			"Las operaciones que AIF no cubre —los ADR (`adr create/edit/import/export`) y el catálogo de etiquetas (`tag create/edit/delete`)— van por la CLI `tasks-ai` convencional; asignar etiquetas va dentro de `create`/`update`.",
		},
		Warnings: []string{
			"Una tarea sin criterio no se empieza: el criterio es lo que después se contrasta con el resultado real.",
			"La épica pasa a `in_progress` cuando empieza su primera hija y no cierra hasta que cierran todas.",
		},
	},
	{
		Name:     "Explorar",
		When:     "antes de proponer o editar, en proporción a la petición",
		Evidence: "las fuentes exploradas: código, requisitos y lo que el cambio debe alcanzar",
		Steps: []string{
			"Leer el código y los requisitos que la petición toca.",
			"En repositorios con `.codegraph/`, `codegraph explore \"...\"` antes de buscar a mano.",
			"Usar `lsp_*` para símbolos, referencias y tipos; `grep` para texto, configuración y documentación.",
			"Mapear lo que el cambio debe alcanzar: llamadores, pruebas, fixtures, configuración y exportaciones.",
		},
		Warnings: []string{
			"Lo razonado sin verificar es inferencia: contrástalo con una herramienta antes de actuar sobre ello.",
		},
	},
	{
		Name:     "Decidir",
		When:     "cuando aparece una duda de diseño o de producto",
		Evidence: "la decisión tomada o «sin decisiones del humano»",
		Steps: []string{
			"Resolver los hechos investigando: repo, herramientas, documentación del paquete.",
			"Elevar solo las decisiones que son del humano: producto, alcance, formato, permisos, pérdida de datos.",
			"Preguntar de una en una y esperar la respuesta.",
			"Formalizar más (un plan, una especificación) es elección explícita de la persona, nunca un cambio silencioso de proceso.",
		},
		Warnings: []string{
			"Ni preguntas innecesarias ni autoridad no concedida.",
			"Puedes advertir que un cambio tiene implicaciones arquitectónicas y proponer formalizarlo; no lo decidas tú.",
		},
	},
	{
		Name:     "Delegar",
		When:     "antes de lanzar un subagente",
		Evidence: "los subagentes lanzados y la tarea de cada uno",
		Steps: []string{
			"Todo encargo lleva cuatro cosas: objetivo, archivos de su propiedad, comprobaciones esperadas y criterio de aceptación; si hay tablero, la referencia de la tarea.",
			"En el encargo, distinguir obstáculo corriente —error, timeout, herramienta que falla—, que se diagnostica y se supera, de bloqueo deliberado —fichero marcado para no tocar, acceso retirado a propósito—, que se respeta y se comunica buscando otra vía segura; el informe de retorno se entiende sin haber visto el trabajo.",
			"Elegir el agente por razón con nombre: `general` para implementar, `explore` para un mapa que exige más de unas cinco consultas, `docs` para documentación y `verify` y `review` para verificación independiente.",
			"El ciclo de cada encargo lo lleva el runner por AIF (`deck_prepare`, `deck_launch`, `deck_return`, `deck_note`, `deck_close`): no hay libro manual ni línea que mantener.",
			"Usar `background` solo para unidades independientes con archivos distintos, y esperar el aviso antes de integrar.",
			"Implementar en línea solo lo trivial, inequívoco y de bajo riesgo, cuando delegar no aporte valor.",
		},
		Warnings: []string{
			"El subagente no toca el tablero ni inventa estados: las escrituras del tablero las hace el orquestador por AIF; una operación lanzada no es evidencia, se comprueba por CLI antes de usarse.",
			"Revisa la salida del subagente antes de integrarla; un encargo sustancial nace además con alcance y criterios en su tarea.",
		},
	},
	{
		Name:     "Verificar",
		When:     "tras cualquier cambio y antes del commit o del cierre",
		Evidence: "el escalón de verificación aplicado y su resultado",
		Steps: []string{
			"Elegir el escalón por alcance y riesgo: trivial → la comprobación localizada que toca ese archivo; funcional → los tests afectados y una prueba del flujo; transversal → la suite relevante o la puerta del repo; crítico → además, verificación independiente.",
			"Partir de la evidencia que ya existe —el `validation` del implementador y el `git diff`— y no repetir una comprobación ya ejecutada sin motivo; `verify` entra cuando falte una comprobación, la independencia o el riesgo lo justifique, y en un cambio crítico su encargo lleva también el objetivo, los criterios y la evidencia disponible: no lo reduzcas a una lista de comandos.",
			"Ejecutar el escalón elegido, no la batería completa por defecto: la batería entera solo cuando el cambio no permita acotar qué puede romper.",
			"Contrastar el criterio de aceptación de la tarea con el resultado real, no con el informe del subagente: el diff y el resumen se contrastan los dos, porque el resumen tiende a omitir cambios pequeños.",
		},
		Warnings: []string{
			"La salida de un subagente no es evidencia: se revisa antes de integrarla.",
			"`review` se reserva a contratos, seguridad, datos, concurrencia o criterio ambiguo; no repite lo ya probado.",
			"Toda verificación no ejecutada se declara.",
		},
	},
	{
		Name:     "Riesgo",
		When:     "en todo cambio de código, antes de preparar el commit",
		Evidence: "la línea `Risk:` escrita",
		Steps: []string{
			"Recorrer los seis ítems: 1 datos o efectos irreversibles; 2 seguridad; 3 contratos que otros consumen; 4 concurrencia; 5 entrega o entorno; 6 nada que detecte una regresión en lo que cambia.",
			"Escribir la línea `Risk: item N (motivo)` si toca alguno, o `Risk: none` si no toca ninguno.",
			"Riesgo alto: añadir verificación independiente, además de las comprobaciones propias; otro agente lee el criterio de la tarea y lo prueba contra el resultado real (`verify` y `review`).",
		},
		Warnings: []string{
			"La línea viaja en el cuerpo del commit: sin ella, el cambio no está cerrado.",
		},
	},
	{
		Name:     "Commit",
		When:     "cuando el criterio está verificado y el trabajo está listo para integrarse",
		Evidence: "la revisión o confirmación de la persona recibida",
		Steps: []string{
			"Preparar el commit y presentar el informe de preparación: qué entra, qué queda fuera y qué comprobaciones se ejecutaron.",
			"Esperar la confirmación explícita: cubre lo presentado, y un cambio de alcance pide otra.",
			"Redactar en Conventional Commits, en inglés, en imperativo y sin punto final, una unidad coherente por commit.",
			"Cerrar el cuerpo con `Refs: tk-n`, una por tarea, y la línea de riesgo.",
			"Seguir `CONTRIBUTING.md` para rama, integración y versiones; `push`, merge y etiqueta piden autorización.",
		},
		Warnings: []string{
			"Sin firmas de IA ni `Co-Authored-By` salvo petición expresa.",
			"El tablero no habla de Git: no escribas estados Git en comentarios del tablero; el estado de Git vive en Git.",
			"Ningún commit con cabos sueltos: lo que aparece al prepararlo se arregla o se pregunta antes.",
		},
	},
	{
		Name:     "Cierre de tarea",
		When:     "tras el commit, al terminar la unidad de trabajo",
		Evidence: "la tarea cerrada o el siguiente paso declarado",
		Steps: []string{
			"Comprobar el criterio de la tarea contra el resultado real.",
			"Dejar la bitácora como comentario en términos del producto: qué se hizo y qué se comprobó, por AIF en primer plano.",
			"Cerrar por AIF en primer plano solo con la revisión de la persona o su pedido expreso.",
			"Dejar el trabajo a medias: devuelve la tarea a `open` y añade un comentario con el siguiente paso, por AIF, y comprueba por CLI que quedó registrado.",
			"Épica: cerrarla cuando cierren todas sus hijas; cerrar con hijas abiertas se rechaza sin escribir nada.",
		},
		Warnings: []string{
			"La bitácora va en términos del producto, sin estados vivos ni estados de Git: ni hashes, ni ramas, ni «mergeado».",
			"`done` no sustituye la revisión: sin ella, la tarea sigue abierta.",
			"Una tarea ya cerrada con `done` no se vuelve a cerrar: reabrirla exige una justificación importante y la autorización explícita de la persona, siempre por AIF sin delegar.",
		},
	},
	{
		Name:     "Memoria y sesión",
		When:     "tras cada decisión, bugfix o hallazgo, y antes de decir «done»",
		Evidence: "ninguna: es la fase terminal y no hay transición que justificar; el cierre del runner lo hace `phase_reset`",
		Steps: []string{
			"Guardar con `acm aif` las operaciones de memoria (decision, learning, discovery, bugfix) tras una decisión, un bugfix o un hallazgo no obvio, agrupado por tema; las preferencias de la persona, su operación `preference`.",
			"Dejar trabajo a medias: `acm aif` con su operación `checkpoint` y el estado con los siguientes pasos.",
			"Antes de decir «done» o cerrar la sesión, `acm aif` con su operación `close` y el resumen del cierre; el cierre del tablero (si el proyecto usa tasks-ai) manda sobre el de memoria.",
			"Antes de dar la sesión por cerrada, `phase_reset` por `runner aif` con su motivo: deja el estado en fase 1 y sin historial, para que una sesión reutilizada no herede la fase colgada; si queda trabajo abierto, la próxima sesión lo recoge desde su tarea.",
		},
		Warnings: []string{
			"Lo que no se guarda se pierde en la próxima sesión.",
			"«Podemos cerrar» de la persona apunta a la sesión, no a la tarea: no repitas el cierre de la fase 10 ni reabras una tarea cerrada sin autorización explícita.",
		},
	},
}

func init() {
	for i := range Phases {
		Phases[i].Number = i + 1
	}
}

// Get returns a copy of phase n, or the first one when out of range.
func Get(n int) Phase {
	if n < 1 || n > Count {
		n = 1
	}
	return Phases[n-1]
}
