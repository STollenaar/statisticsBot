package helpcommand

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/stollenaar/statisticsbot/internal/util"
)

var HelpCmd = HelpCommand{
	Name:        "help",
	Description: "Show the available commands and how to use them",
}

type HelpCommand struct {
	Name                string
	Description         string
	ApplicationCommands *[]discord.ApplicationCommandCreate
}

func (h HelpCommand) CreateCommandArguments() []discord.ApplicationCommandOption {
	return nil
}

func (h HelpCommand) Handler(event *events.ApplicationCommandInteractionCreate) {
	if err := event.DeferCreateMessage(true); err != nil {
		slog.Error("Error deferring help command", slog.Any("err", err))
		return
	}
	util.UpdateInteractionResponse(event, helpOverviewComponents())
}

func (h HelpCommand) ComponentHandler(event *events.ComponentInteractionCreate) {
	if err := event.DeferUpdateMessage(); err != nil {
		slog.Error("Error deferring help component", slog.Any("err", err))
		return
	}

	var components []discord.LayoutComponent
	switch event.Data.CustomID() {
	case "help_select":
		values := event.StringSelectMenuInteractionData().Values
		if len(values) == 0 {
			components = helpOverviewComponents()
		} else {
			components = helpDetailComponents(values[0])
		}
	case "help_overview":
		components = helpOverviewComponents()
	default:
		components = helpOverviewComponents()
	}
	util.UpdateComponentInteractionResponse(event, components)
}

type helpEntry struct {
	name        string
	description string
	options     []discord.ApplicationCommandOption
}

func helpEntries() []helpEntry {
	if HelpCmd.ApplicationCommands == nil {
		return nil
	}
	cmds := *HelpCmd.ApplicationCommands
	entries := make([]helpEntry, 0, len(cmds))
	for _, cmd := range cmds {
		slash, ok := cmd.(*discord.SlashCommandCreate)
		if !ok {
			continue
		}
		entries = append(entries, helpEntry{
			name:        slash.Name,
			description: slash.Description,
			options:     slash.Options,
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].name < entries[j].name })
	return entries
}

func findHelpEntry(name string) (helpEntry, bool) {
	for _, e := range helpEntries() {
		if e.name == name {
			return e, true
		}
	}
	return helpEntry{}, false
}

func helpOverviewComponents() []discord.LayoutComponent {
	entries := helpEntries()

	var lines []string
	lines = append(lines,
		"# Statistics Bot — Help",
		"Pick a command from the menu below to see its full usage.",
		"",
	)
	for _, e := range entries {
		lines = append(lines, fmt.Sprintf("- `/%s` — %s", e.name, e.description))
	}

	return []discord.LayoutComponent{
		discord.ContainerComponent{
			Components: []discord.ContainerSubComponent{
				discord.TextDisplayComponent{Content: strings.Join(lines, "\n")},
				util.GetSeparator(),
				discord.ActionRowComponent{
					Components: []discord.InteractiveComponent{
						helpSelectMenu(entries, "", "Pick a command to view details"),
					},
				},
			},
		},
	}
}

func helpDetailComponents(name string) []discord.LayoutComponent {
	entry, ok := findHelpEntry(name)
	if !ok {
		return helpOverviewComponents()
	}

	var sections []string
	sections = append(sections, fmt.Sprintf("# /%s", entry.name))
	sections = append(sections, entry.description)
	if body := renderOptionsBody(entry.name, entry.options); body != "" {
		sections = append(sections, body)
	}

	entries := helpEntries()
	return []discord.LayoutComponent{
		discord.ContainerComponent{
			Components: []discord.ContainerSubComponent{
				discord.TextDisplayComponent{Content: strings.Join(sections, "\n\n")},
				util.GetSeparator(),
				discord.ActionRowComponent{
					Components: []discord.InteractiveComponent{
						helpSelectMenu(entries, entry.name, "Pick another command"),
					},
				},
				discord.ActionRowComponent{
					Components: []discord.InteractiveComponent{
						discord.ButtonComponent{
							Style:    discord.ButtonStyleSecondary,
							Label:    "← Back to overview",
							CustomID: "help_overview",
						},
					},
				},
			},
		},
	}
}

func helpSelectMenu(entries []helpEntry, selected, placeholder string) discord.StringSelectMenuComponent {
	options := make([]discord.StringSelectMenuOption, 0, len(entries))
	for _, e := range entries {
		options = append(options, discord.StringSelectMenuOption{
			Label:       "/" + e.name,
			Value:       e.name,
			Description: truncate(e.description, 100),
			Default:     e.name == selected,
		})
	}
	return discord.StringSelectMenuComponent{
		CustomID:    "help_select",
		Placeholder: placeholder,
		Options:     options,
	}
}

func renderOptionsBody(cmdName string, options []discord.ApplicationCommandOption) string {
	if len(options) == 0 {
		return ""
	}

	hasSub := false
	for _, opt := range options {
		switch opt.Type() {
		case discord.ApplicationCommandOptionTypeSubCommand,
			discord.ApplicationCommandOptionTypeSubCommandGroup:
			hasSub = true
		}
	}

	var sb strings.Builder
	if hasSub {
		sb.WriteString("## Subcommands\n")
		for _, opt := range options {
			switch o := opt.(type) {
			case discord.ApplicationCommandOptionSubCommandGroup:
				fmt.Fprintf(&sb, "\n**%s** — %s\n", o.Name, o.Description)
				for _, sub := range o.Options {
					fmt.Fprintf(&sb, "- `/%s %s %s` — %s\n",
						cmdName, o.Name, sub.Name, sub.Description)
					writeOptionLines(&sb, sub.Options, "    ")
				}
			case discord.ApplicationCommandOptionSubCommand:
				fmt.Fprintf(&sb, "- `/%s %s` — %s\n", cmdName, o.Name, o.Description)
				writeOptionLines(&sb, o.Options, "    ")
			}
		}
	} else {
		sb.WriteString("## Options\n")
		writeOptionLines(&sb, options, "")
	}

	return strings.TrimRight(sb.String(), "\n")
}

func writeOptionLines(sb *strings.Builder, options []discord.ApplicationCommandOption, indent string) {
	for _, opt := range options {
		name, typeName, required := optionMeta(opt)
		marker := ""
		if required {
			marker = ", required"
		}
		fmt.Fprintf(sb, "%s- `%s` *(%s%s)* — %s\n",
			indent, name, typeName, marker, opt.OptionDescription())
	}
}

func optionMeta(opt discord.ApplicationCommandOption) (name, typeName string, required bool) {
	name = opt.OptionName()
	switch o := opt.(type) {
	case discord.ApplicationCommandOptionString:
		typeName, required = "string", o.Required
	case discord.ApplicationCommandOptionInt:
		typeName, required = "integer", o.Required
	case discord.ApplicationCommandOptionBool:
		typeName, required = "boolean", o.Required
	case discord.ApplicationCommandOptionUser:
		typeName, required = "user", o.Required
	case discord.ApplicationCommandOptionChannel:
		typeName, required = "channel", o.Required
	case discord.ApplicationCommandOptionRole:
		typeName, required = "role", o.Required
	case discord.ApplicationCommandOptionMentionable:
		typeName, required = "mentionable", o.Required
	case discord.ApplicationCommandOptionFloat:
		typeName, required = "float", o.Required
	case discord.ApplicationCommandOptionAttachment:
		typeName, required = "attachment", o.Required
	case discord.ApplicationCommandOptionSubCommand:
		typeName = "subcommand"
	case discord.ApplicationCommandOptionSubCommandGroup:
		typeName = "group"
	default:
		typeName = "unknown"
	}
	return
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 1 {
		return s[:max]
	}
	return s[:max-1] + "…"
}
