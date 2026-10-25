package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

// ==========================================
// [ SECTION 1: CONFIGURATION & PALETTE ]
// ==========================================

const (
	repoOwner  = "YoungMind1"
	repoName   = "termu"
	dataBranch = "data"
)

var (
	goldColor   = lipgloss.Color("#FCD34D")
	amberColor  = lipgloss.Color("#F59E0B")
	darkColor   = lipgloss.Color("#0F172A")
	mutedColor  = lipgloss.Color("#94A3B8")
	dangerColor = lipgloss.Color("#EF4444")

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(amberColor).
			MarginLeft(2)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			Faint(true)

	cardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(amberColor).
			Padding(0, 1).
			MarginLeft(2)

	spinnerStyle = lipgloss.NewStyle().
			Foreground(amberColor)
)

// ==========================================
// [ SECTION 2: DATA STRUCTURES ]
// ==========================================

type Student struct {
	Name     string `json:"name"`
	Scores   []int  `json:"scores"`
	Absences int    `json:"absences"`
}

// CommaScores formats the slice of scores as a comma-separated string (e.g. "94,98,96").
func (s Student) CommaScores() string {
	strs := make([]string, len(s.Scores))
	for i, v := range s.Scores {
		strs[i] = strconv.Itoa(v)
	}
	return strings.Join(strs, ",")
}

// ==========================================
// [ SECTION 3: INTERACTIVE HELP CONTENT ]
// ==========================================

var helpGuides = map[string]string{
	"basic": `
# 🐧 Basic Listing

Prints student names in an interactive card view when run directly in a terminal.
When redirected or piped to another command, formatting is stripped and plain text lines are emitted.

### 💡 Usage
` + "```bash" + `
termu
` + "```" + `
`,

	"score": `
# 🎯 Score Flags (--score & --scores)

Appends dummy score data to the output.

* **--score**: Outputs latest score per student.
* **--scores**: Outputs all individual exam scores separated by commas (e.g., 94,98,96).

### 📋 Piped Format
When piped, columns are strictly separated by a **TAB (\t)** character:
` + "`Name <TAB> Score`" + `

### 💡 Usage
` + "```bash" + `
termu --score
termu --scores
` + "```" + `
`,

	"absence": `
# ⚠️ Absence Flag (--absence)

Appends the absence count for each record. Highlights counts over 3 in red inside the interactive terminal.

### 📋 Piped Format
When piped, columns are strictly separated by a **TAB (\t)** character:
* Alone: ` + "`Name <TAB> Absences`" + `
* Combined: ` + "`Name <TAB> Score <TAB> Absences`" + `

### 💡 Usage
` + "```bash" + `
termu --absence
termu --score --absence
` + "```" + `
`,

	"live": `
# 📡 Live Stream (termu live)

Continuously emits simulated lab activity logs to stdout every second until interrupted (Ctrl+C).

### 📋 Format
` + "`Timestamp <TAB> Name <TAB> Event`" + `

### 💡 Usage
` + "```bash" + `
termu live
` + "```" + `
`,
}

func launchInteractiveHelp() {
	var selectedTopic string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("🐧 Termu Command Reference").
				Description("Select a topic to inspect its flags and output structure:").
				Options(
					huh.NewOption("1. Basic Listing (termu)", "basic"),
					huh.NewOption("2. Score Flags (--score / --scores)", "score"),
					huh.NewOption("3. Absence Flag (--absence)", "absence"),
					huh.NewOption("4. Live Activity Stream (termu live)", "live"),
				).
				Value(&selectedTopic),
		),
	).WithTheme(huh.ThemeCharm())

	if err := form.Run(); err != nil {
		fmt.Println("Selection cancelled.")
		return
	}

	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)
	if err != nil {
		fmt.Println(helpGuides[selectedTopic])
		return
	}

	renderedMarkdown, err := renderer.Render(helpGuides[selectedTopic])
	if err != nil {
		fmt.Println(helpGuides[selectedTopic])
		return
	}

	fmt.Print(renderedMarkdown)
}

// ==========================================
// [ SECTION 4: MAIN EXECUTION ]
// ==========================================

func main() {
	showScore := flag.Bool("score", false, "Display latest score")
	showScores := flag.Bool("scores", false, "Display comma-separated exam scores")
	showAbsence := flag.Bool("absence", false, "Display absence tally")
	showHelp := flag.Bool("help", false, "Interactive help menu")

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "help":
			launchInteractiveHelp()
			return
		case "live":
			students := fetchStudents()

			runLiveStream(students)
			return
		}
	}

	flag.Usage = func() {
		launchInteractiveHelp()
	}

	flag.Parse()

	if *showHelp {
		launchInteractiveHelp()
		return
	}

	students := fetchStudents()

	isInteractive := term.IsTerminal(int(os.Stdout.Fd()))

	if isInteractive {
		showSpinnerAnimation()
		renderMascotBanner()
		renderStyledList(students, *showScore, *showScores, *showAbsence)
	} else {
		// Strict TAB-delimited output for clean Unix piping
		for _, s := range students {
			cols := []string{s.Name}
			if *showScore {
				cols = append(cols, fmt.Sprintf("%d", s.Scores[len(s.Scores)-1]))
			}
			if *showScores {
				cols = append(cols, s.CommaScores())
			}
			if *showAbsence {
				cols = append(cols, fmt.Sprintf("%d", s.Absences))
			}
			fmt.Println(strings.Join(cols, "\t"))
		}
	}
}

// ==========================================
// [ SECTION 5: GITHUB DATA FETCH & STREAM ]
// ==========================================

func fetchStudents() []Student {
	url := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/students.json", repoOwner, repoName, dataBranch)

	client := &http.Client{Timeout: 4 * time.Second}
	resp, err := client.Get(url)
	if err == nil && resp.StatusCode == http.StatusOK {
		defer resp.Body.Close()
		var remoteStudents []Student
		if err := json.NewDecoder(resp.Body).Decode(&remoteStudents); err == nil && len(remoteStudents) > 0 {
			return remoteStudents
		}
	}

	// Offline / Unconfigured Fallback Dataset (Dummy Scores)
	return []Student{
		{Name: "Reza Esmaeilzadeh", Scores: []int{92, 95, 94}, Absences: 0},
		{Name: "Rozhin Anousha", Scores: []int{89, 93, 91}, Absences: 1},
		{Name: "Nima Anoushan", Scores: []int{94, 90, 96}, Absences: 0},
		{Name: "Donald Knuth", Scores: []int{74, 70, 77}, Absences: 6},
		{Name: "Mohaddeseh Babaei Sarj", Scores: []int{88, 92, 90}, Absences: 1},
		{Name: "Nima Bakhor", Scores: []int{91, 87, 93}, Absences: 2},
		{Name: "Sayman Bagherzadeh", Scores: []int{95, 98, 96}, Absences: 0},
		{Name: "Amin Pakjoo", Scores: []int{86, 91, 89}, Absences: 1},
		{Name: "Seyed Araz Pouraghdam", Scores: []int{93, 95, 92}, Absences: 0},
		{Name: "Aylin Tabrizi", Scores: []int{97, 94, 98}, Absences: 0},
		{Name: "Amir Abolfazl Jafarzadeh", Scores: []int{89, 86, 91}, Absences: 2},
		{Name: "Arian Harbi", Scores: []int{92, 94, 90}, Absences: 1},
		{Name: "Mobin Hasani", Scores: []int{90, 93, 95}, Absences: 0},
		{Name: "Amir Khezri", Scores: []int{87, 90, 88}, Absences: 1},
		{Name: "Aynaz Dorostkar Rouhani", Scores: []int{96, 92, 95}, Absences: 0},
		{Name: "Ahoura Saeidpour", Scores: []int{91, 94, 93}, Absences: 1},
		{Name: "Seyed Yaser Seyedi Taljar", Scores: []int{88, 91, 87}, Absences: 2},
		{Name: "Amir Mohammad Shokri", Scores: []int{94, 97, 95}, Absences: 0},
		{Name: "Shayan Abbaszadeh", Scores: []int{89, 92, 91}, Absences: 1},
		{Name: "Taha Eyvazi", Scores: []int{93, 89, 94}, Absences: 0},
		{Name: "Soma Ghaderi", Scores: []int{95, 96, 98}, Absences: 0},
		{Name: "Nazanin Ghasemi", Scores: []int{91, 93, 90}, Absences: 1},
		{Name: "Amir Ehsan Motazaker", Scores: []int{87, 89, 92}, Absences: 2},
		{Name: "Ada Lovelace", Scores: []int{82, 79, 84}, Absences: 4},
		{Name: "Matin Majidi", Scores: []int{94, 91, 95}, Absences: 0},
		{Name: "Amir Hossein Mohammadi", Scores: []int{90, 94, 92}, Absences: 1},
		{Name: "Hesam Mollazadeh", Scores: []int{88, 86, 90}, Absences: 1},
		{Name: "Meysam Nasrollahzadeh", Scores: []int{92, 95, 93}, Absences: 0},
		{Name: "Nahal Nichalani", Scores: []int{97, 95, 99}, Absences: 0},
		{Name: "Reyhaneh Vahabi Rad", Scores: []int{89, 93, 91}, Absences: 1},
		{Name: "Sadra Karimi Baloo", Scores: []int{93, 90, 94}, Absences: 0},
		{Name: "Nima Faghih", Scores: []int{86, 89, 88}, Absences: 2},
		{Name: "Amir Reza Kamali Aghdam", Scores: []int{95, 92, 96}, Absences: 0},
		{Name: "Araz Ahmadi Khoyi", Scores: []int{91, 94, 92}, Absences: 1},
		{Name: "Fatemeh Asgharzadeh", Scores: []int{96, 98, 95}, Absences: 0},
		{Name: "Hanieh Aghgonbadi", Scores: []int{89, 91, 93}, Absences: 1},
		{Name: "Milad Anvari Gharabaghloo", Scores: []int{87, 90, 89}, Absences: 2},
		{Name: "Mohammad Ounagh", Scores: []int{92, 94, 91}, Absences: 0},
		{Name: "Mahsa Bostani", Scores: []int{94, 96, 93}, Absences: 0},
		{Name: "Samin Besharat Rezaei", Scores: []int{90, 88, 92}, Absences: 1},
		{Name: "Nadia Parsa Chongharaloo", Scores: []int{95, 93, 97}, Absences: 0},
		{Name: "Ayda Partovi Gharabagh", Scores: []int{88, 92, 90}, Absences: 1},
		{Name: "Ozra Pazhouhi", Scores: []int{91, 89, 94}, Absences: 1},
		{Name: "Taha Jafarzadeh Zangalani", Scores: []int{93, 95, 91}, Absences: 0},
		{Name: "Mohammad Reza Jangi Ghayeh Bolaghi", Scores: []int{87, 91, 88}, Absences: 2},
		{Name: "Mahlagha Hanifi", Scores: []int{96, 94, 97}, Absences: 0},
		{Name: "Sevil Khorramdel", Scores: []int{90, 93, 92}, Absences: 1},
		{Name: "Melisa Rahimi Adl", Scores: []int{94, 92, 95}, Absences: 0},
		{Name: "Golsa Razzaghi Tapeh", Scores: []int{89, 87, 91}, Absences: 1},
		{Name: "Parnia Rasoulian", Scores: []int{97, 96, 98}, Absences: 0},
		{Name: "Hasan Salari Yaghlan Tapeh", Scores: []int{86, 90, 88}, Absences: 2},
		{Name: "Sonay Shahmohammadi Ousaloo", Scores: []int{92, 95, 94}, Absences: 0},
		{Name: "Mohammad Sadeghi Topragh Ghaleh", Scores: []int{88, 91, 90}, Absences: 1},
		{Name: "Amir Reza Sadeghi Siahkali", Scores: []int{93, 94, 96}, Absences: 0},
		{Name: "Masoumeh Taherabadi", Scores: []int{91, 89, 92}, Absences: 1},
		{Name: "Sama Alizadeh", Scores: []int{95, 97, 94}, Absences: 0},
		{Name: "Shilan Farhang", Scores: []int{89, 92, 90}, Absences: 1},
		{Name: "Nesa Mohammadpour", Scores: []int{94, 91, 93}, Absences: 0},
		{Name: "Linus Torvalds", Scores: []int{72, 68, 75}, Absences: 5},
		{Name: "Farinaz Mohammadzadeh", Scores: []int{87, 89, 91}, Absences: 2},
		{Name: "Seyed Mojtaba Mahmoudi", Scores: []int{92, 96, 94}, Absences: 0},
		{Name: "Elina Marandi", Scores: []int{96, 95, 98}, Absences: 0},
		{Name: "Marjan Mashhadi", Scores: []int{90, 92, 89}, Absences: 1},
		{Name: "Sina Maghsoudi", Scores: []int{93, 91, 95}, Absences: 0},
		{Name: "Arian Nabizadeh Zolpirani", Scores: []int{88, 90, 87}, Absences: 2},
		{Name: "Parsa Nazm Kordlar", Scores: []int{95, 94, 96}, Absences: 0},
		{Name: "Parham Naghinezhad", Scores: []int{91, 93, 92}, Absences: 1},
		{Name: "Ali Nouri", Scores: []int{94, 97, 95}, Absences: 0},
		{Name: "Ken Thompson", Scores: []int{78, 81, 76}, Absences: 4},
	}
}

func runLiveStream(students []Student) {
	events := []string{"LOGIN_SSH", "SUBMIT_LAB", "COMPILE_OK", "PIPE_CREATED", "EXIT_0"}
	for {
		s := students[rand.Intn(len(students))]
		ev := events[rand.Intn(len(events))]
		ts := time.Now().Format("15:04:05")
		fmt.Printf("%s\t%s\t%s\n", ts, s.Name, ev)
		time.Sleep(1 * time.Second)
	}
}

// ==========================================
// [ SECTION 6: ANIMATION & RENDERING ]
// ==========================================

func showSpinnerAnimation() {
	s := spinner.Dot
	fmt.Print("\n  ")
	for i := 0; i < 12; i++ {
		frame := s.Frames[i%len(s.Frames)]
		fmt.Printf("\r  %s %s", spinnerStyle.Render(frame), subtitleStyle.Render("Syncing Termu roster..."))
		time.Sleep(35 * time.Millisecond)
	}
	fmt.Print("\r\033[K")
}

func renderMascotBanner() {
	mascotLines := []string{
		lipgloss.NewStyle().Foreground(goldColor).Render("       ▄█████████▄       "),
		lipgloss.NewStyle().Foreground(goldColor).Render("    ▄███████████████▄    "),
		lipgloss.NewStyle().Foreground(amberColor).Render("   ███  ") + lipgloss.NewStyle().Foreground(darkColor).Bold(true).Render("▀███████▀") + lipgloss.NewStyle().Foreground(amberColor).Render("  ███   "),
		lipgloss.NewStyle().Foreground(amberColor).Render("  █████▄ ") + lipgloss.NewStyle().Foreground(darkColor).Bold(true).Render("▀█████▀") + lipgloss.NewStyle().Foreground(amberColor).Render(" ▄█████  "),
		lipgloss.NewStyle().Foreground(amberColor).Render("   ███████ ") + lipgloss.NewStyle().Foreground(darkColor).Render("▀█▀") + lipgloss.NewStyle().Foreground(amberColor).Render(" ███████   "),
		lipgloss.NewStyle().Foreground(amberColor).Render("    ▀███████████████▀    "),
	}

	mascotArt := strings.Join(mascotLines, "\n")
	headerText := fmt.Sprintf("%s\n%s",
		titleStyle.Render("T E R M U"),
		subtitleStyle.Render("  The Sovereign Linux Companion • v1.1.0\n  Tip: Run 'termu help' for documentation"),
	)

	combined := lipgloss.JoinHorizontal(lipgloss.Center, mascotArt, headerText)
	fmt.Println("\n" + combined + "\n")
}

func renderStyledList(students []Student, showScore, showScores, showAbsence bool) {
	var rows []string

	for _, s := range students {
		name := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Bold(true).Width(40).Render(s.Name)
		row := lipgloss.NewStyle().Foreground(amberColor).Render("● ") + name

		if showScore {
			scoreText := fmt.Sprintf("%d pts", s.Scores[len(s.Scores)-1])
			styledScore := lipgloss.NewStyle().Foreground(goldColor).Width(12).Render(scoreText)
			row = lipgloss.JoinHorizontal(lipgloss.Left, row, styledScore)
		}

		if showScores {
			scoresText := fmt.Sprintf("[%s]", s.CommaScores())
			styledScores := lipgloss.NewStyle().Foreground(goldColor).Width(16).Render(scoresText)
			row = lipgloss.JoinHorizontal(lipgloss.Left, row, styledScores)
		}

		if showAbsence {
			absText := fmt.Sprintf("%d absences", s.Absences)
			st := lipgloss.NewStyle().Foreground(mutedColor).Width(14)
			if s.Absences > 3 {
				st = lipgloss.NewStyle().Foreground(dangerColor).Bold(true).Width(14)
			}
			row = lipgloss.JoinHorizontal(lipgloss.Left, row, st.Render(absText))
		}

		rows = append(rows, row)
	}

	fmt.Println(cardStyle.Render(strings.Join(rows, "\n")))
	fmt.Println()
}

func fetchRemoteText(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil || resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("network issue")
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return string(body), nil
}
