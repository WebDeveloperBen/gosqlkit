package cli

import (
	"errors"
	"fmt"
	"io"
)

type CompletionCmd struct {
	Shell string `arg:"" help:"Shell to generate completion for: bash, zsh, fish, or powershell."`
}

func (c *CompletionCmd) Run(g *GlobalFlags) error {
	var script string
	switch c.Shell {
	case "bash":
		script = bashCompletion
	case "zsh":
		script = zshCompletion
	case "fish":
		script = fishCompletion
	case "powershell", "pwsh":
		script = powershellCompletion
	default:
		return Exit(1, errors.New("unsupported shell; choose bash, zsh, fish, or powershell"))
	}
	if _, err := io.WriteString(g.stdout(), script); err != nil {
		return Exit(1, fmt.Errorf("write completion script: %w", err))
	}
	return nil
}

const bashCompletion = `# bash completion for gosqlkit
_gosqlkit_completion() {
    local commands="version ci inspect drift snapshot generate migrate completion"
    local cur="${COMP_WORDS[COMP_CWORD]}"
    if [[ ${COMP_CWORD} -eq 1 ]]; then
        COMPREPLY=($(compgen -W "${commands}" -- "${cur}"))
        return
    fi
    if [[ ${COMP_WORDS[1]} == completion && ${COMP_CWORD} -eq 2 ]]; then
        COMPREPLY=($(compgen -W "bash zsh fish powershell" -- "${cur}"))
        return
    fi
}
complete -F _gosqlkit_completion gosqlkit
`

const zshCompletion = `#compdef gosqlkit
_gosqlkit() {
  _arguments '1:command:(version ci inspect drift snapshot generate migrate completion)' \
    '*::argument:->args'
  case $words[2] in
    completion) _values 'shell' bash zsh fish powershell ;;
  esac
}
_gosqlkit
`

const fishCompletion = `# fish completion for gosqlkit
complete -c gosqlkit -f -n '__fish_use_subcommand' -a 'version ci inspect drift snapshot generate migrate completion'
complete -c gosqlkit -f -n '__fish_seen_subcommand_from completion' -a 'bash zsh fish powershell'
`

const powershellCompletion = `# PowerShell completion for gosqlkit
Register-ArgumentCompleter -Native -CommandName gosqlkit -ScriptBlock {
    param($wordToComplete, $commandAst, $cursorPosition)
    $commands = 'version','ci','inspect','drift','snapshot','generate','migrate','completion'
    if ($commandAst.CommandElements.Count -le 1) {
        $commands | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
            [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
        }
    } elseif ($commandAst.CommandElements[1].Value -eq 'completion') {
        'bash','zsh','fish','powershell' | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
            [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
        }
    }
}
`
