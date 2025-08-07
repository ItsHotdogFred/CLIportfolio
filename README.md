# 🚀 CLI Portfolio

To access the demo you can go to either
- ssh -p 1111 terminal.itsfred.dev | For Fred CLI aka my portfolio
- ssh -p 2222 terminal.itsfred.dev | For cli based wikipedia

An interactive command-line portfolio application built with Go, featuring a Wikipedia CLI tool and personal portfolio interface accessible via SSH.

## 📋 Features

### 🎯 Interactive Portfolio CLI
- **Beautiful TUI Interface**: Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) for smooth terminal interactions
- **SSH Server**: Connect remotely via SSH on `localhost:2222`
- **File Navigation**: Browse through projects and personal information
- **Interactive Commands**: Responsive command-line interface with history

### 📚 Wikipedia CLI Tool  
- **Real-time Search**: Search Wikipedia articles directly from the terminal
- **Beautiful Formatting**: Styled output with summaries and full content
- **SSH Access**: Available via SSH on `localhost:234`
- **Responsive Design**: Viewport with scrolling and navigation controls

## 🛠️ Tech Stack

- **Language**: Go 1.21+
- **UI Framework**: [Bubble Tea](https://github.com/charmbracelet/bubbletea) - Terminal User Interface
- **SSH Server**: [Wish](https://github.com/charmbracelet/wish) - SSH server framework
- **Styling**: [Lipgloss](https://github.com/charmbracelet/lipgloss) - Terminal styling
- **Wikipedia API**: [go-wiki](https://github.com/trietmn/go-wiki) - Wikipedia integration

## 🚀 Quick Start

### Prerequisites
- Go 1.21 or later
- Git

### Installation

1. **Clone the repository**
   ```bash
   git clone https://github.com/ItsHotdogFred/CLIportfolio.git
   cd CLIportfolio
   ```

2. **Install dependencies**
   ```bash
   # For the main portfolio
   cd Portfolio
   go mod tidy
   
   # For the Wikipedia CLI
   cd ../WikipediaCLI
   go mod tidy
   ```

3. **Run the Portfolio CLI**
   ```bash
   cd Portfolio
   go run main.go
   ```

4. **Run the Wikipedia CLI**
   ```bash
   cd WikipediaCLI
   go run main.go
   ```

### 🔌 SSH Access

#### Portfolio CLI
```bash
ssh localhost -p 2222
```

#### Wikipedia CLI
```bash
ssh localhost -p 234
```

## 📖 Usage

### Portfolio CLI Navigation
- Use arrow keys to navigate
- Press `Enter` to select items
- Type commands to interact with the system
- Press `q` or `Ctrl+C` to quit

### Wikipedia CLI
- Enter search queries in the input field
- Press `Enter` to search
- Use arrow keys to scroll through results
- Press `ESC` to return to search
- Press `q` to quit

## 🎮 About the Developer

I'm a passionate developer who loves creating games using GDScript in the Godot Game Engine. I enjoy building custom systems that make life easier and have released several games on itch.io.

### 🎯 Current Projects
- **Repulsus Insania**: A Celeste-inspired platformer for Hackclub Jumpstart
- **Pixelator**: My first released game - a challenging pixel art adventure

### 🎨 Games Portfolio
- [Pixelator](https://itshotdogfred.itch.io/pixelator) - My debut game on itch.io

## 📁 Project Structure

```
CLIportfolio/
├── Portfolio/              # Main portfolio CLI application
│   ├── main.go            # Portfolio server and TUI
│   ├── bio.txt            # Personal bio
│   ├── contact.txt        # Contact information  
│   ├── go.mod             # Go dependencies
│   └── Projects/          # Project descriptions
│       ├── jumpstart-gameidea.md
│       └── Pixelator.md
├── WikipediaCLI/          # Wikipedia search tool
│   ├── main.go            # Wikipedia CLI application
│   └── go.mod             # Go dependencies
└── README.md              # This file
```

## 🤝 Contributing

Contributions are welcome! Feel free to:
- Report bugs
- Suggest new features
- Submit pull requests
- Improve documentation

## 📞 Contact

- **GitHub**: [github.com/ItsHotdogFred](https://github.com/ItsHotdogFred)
- **Itch.io**: [itshotdogfred.itch.io](https://itshotdogfred.itch.io)
- **Email**: cli@itsfred.dev

## 📄 License

This project is open source and available under the [MIT License](LICENSE).

## 🙏 Acknowledgments

- [Charm](https://charm.sh/) for the amazing TUI libraries
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) community
- [go-wiki](https://github.com/trietmn/go-wiki) for Wikipedia integration

---

*Built with ❤️ using Go and Bubble Tea*
