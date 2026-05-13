use gemini_crate::client::GeminiClient;
use serde::{Deserialize, Serialize};
use std::fs;
use std::io::{self, Write};
use termsize;

const HISTORY_FILE: &str = "chat_history.json";
const MAX_LINE_CHARS: usize = 120;

#[derive(Serialize, Deserialize, Clone, Debug)]
struct Part { text: Option<String> }

#[derive(Serialize, Deserialize, Clone, Debug)]
struct HistoryEntry { role: String, parts: Vec<Part> }

struct FormatterState {
    bold_active: bool,
    italic_active: bool,
    line_length: usize,
    in_code_block: Option<String>,
}

impl FormatterState {
    fn new() -> Self {
        Self { bold_active: false, italic_active: false, line_length: 0, in_code_block: None }
    }
}

fn get_padding() -> String {
    let width = termsize::get().map(|s| s.cols).unwrap_or(140) as f64;
    let p = ((width - MAX_LINE_CHARS as f64 - 10.0) * 0.1) as usize;
    " ".repeat(p)
}

fn print_formatted(text: &str, indent: &str, state: &mut FormatterState) {
    let lines: Vec<&str> = text.lines().collect();
    for (idx, line) in lines.iter().enumerate() {
        let mut current_line = line.trim_end().to_string();

        if current_line.starts_with('|') {
            if current_line.contains("---") || current_line.contains(":---") { continue; }
            current_line = format!("\x1b[2m{}\x1b[0m", current_line.replace('|', "\x1b[37m│\x1b[2m"));
        }

        let color_map = [
            ("magenta", "35"), ("cyan", "36"), ("gold", "33"), ("purple", "35"), 
            ("red", "31"), ("green", "32"), ("blue", "34"), ("teal", "36"), 
            ("orange", "33"), ("pink", "95"), ("maroon", "31"), ("lime", "92"), 
            ("navy", "34"), ("salmon", "91"), ("grey", "90"), ("gray", "90"),
            ("lightgray", "37"), ("silver", "37"), ("olive", "32"), ("brown", "33")
        ];

        if current_line.contains("$\\color{") {
            for (name, code) in color_map {
                let start_pattern = format!("$\\color{{{}}}{{\\text{{", name);
                if current_line.contains(&start_pattern) {
                    current_line = current_line.replace(&start_pattern, &format!("\x1b[{}m", code));
                }
            }
        }
        
        if current_line.contains("}}$") {
            current_line = current_line.replace("}}$", "\x1b[39m");
        }

        if current_line.starts_with("```") {
            if state.in_code_block.is_none() {
                let lang = current_line.replace("
```", "").trim().to_string();
                state.in_code_block = Some(if lang.is_empty() { "text".into() } else { lang });
                println!("\x1b[2m--- {} ---\x1b[0m", state.in_code_block.as_ref().unwrap().to_uppercase());
                print!("{}", indent);
            } else {
                state.in_code_block = None;
                println!("\x1b[2m-----------------------\x1b[0m");
                if idx < lines.len() - 1 { print!("{}", indent); }
            }
            continue;
        }

        if let Some(_) = state.in_code_block {
            println!("\x1b[37m{}\x1b[0m", current_line);
            if idx < lines.len() - 1 { print!("{}", indent); }
            continue;
        }

        let chars: Vec<char> = current_line.chars().collect();
        let mut i = 0;
        state.line_length = 0;
        while i < chars.len() {
            let c = chars[i];
            if c == '\x1b' {
                while i < chars.len() && chars[i] != 'm' { print!("{}", chars[i]); i += 1; }
                if i < chars.len() { print!("m"); i += 1; }
                continue;
            }
            if c == '*' {
                if i + 1 < chars.len() && chars[i + 1] == '*' {
                    print!("{}", if state.bold_active { "\x1b[22m" } else { "\x1b[1m" });
                    state.bold_active = !state.bold_active;
                    i += 2; continue;
                } else {
                    print!("{}", if state.italic_active { "\x1b[23m" } else { "\x1b[3m" });
                    state.italic_active = !state.italic_active;
                    i += 1; continue;
                }
            }
            if state.line_length >= MAX_LINE_CHARS && c.is_whitespace() {
                print!("\n{}", indent);
                state.line_length = 0;
            } else {
                print!("{}", c);
                state.line_length += 1;
            }
            i += 1;
        }
        print!("\x1b[0m");
        if idx < lines.len() - 1 { println!(); print!("{}", indent); }
    }
    io::stdout().flush().unwrap();
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let client = GeminiClient::new().map_err(|_| "GEMINI_API_KEY env var missing")?;
    let mut history = load_history();
    let p = get_padding();
    let indent = format!("{}        ", p);

    for entry in &history {
        let text = entry.parts[0].text.as_deref().unwrap_or("");
        if entry.role == "user" {
            println!("\n{}\x1b[36mYou > \x1b[0m{}", p, text);
        } else {
            print!("{}\x1b[35mGemini:\x1b[0m ", p);
            print_formatted(text, &indent, &mut FormatterState::new());
            println!();
        }
    }

    println!("\n{}\x1b[33m--- Gemini Rust CLI v2.0 (Master Edition) ---\x1b[0m\n", p);

	loop {
        print!("{}\x1b[36mYou > \x1b[0m", p);
        io::stdout().flush()?;
        let mut input = String::new();
        io::stdin().read_line(&mut input)?;
        let input = input.trim();
        if input == "exit" || input == "quit" { break; }
        if input.is_empty() { continue; }

		print!("{}\x1b[35m[ Thinking... ]\x1b[0m", p);
        io::stdout().flush()?;

        let mut full_prompt = String::new();
        for entry in &history {
            full_prompt.push_str(&format!("{}: {}\n", if entry.role == "user" { "User" } else { "Model" }, entry.parts[0].text.as_deref().unwrap_or("")));
        }
        full_prompt.push_str(&format!("User: {}\nModel: ", input));

        match client.generate_text("gemini-3-flash-preview", &full_prompt).await {
            Ok(resp) => {
                let text = resp.to_string();
                print!("\r{}\x1b[35mGemini:\x1b[0m ", p);
                print_formatted(&text, &indent, &mut FormatterState::new());
                println!("\n");
                history.push(HistoryEntry { role: "user".into(), parts: vec![Part { text: Some(input.into()) }] });
                history.push(HistoryEntry { role: "model".into(), parts: vec![Part { text: Some(text) }] });
                save_history(&history);
            }
            Err(e) => { print!("\r"); eprintln!("{}Error: {:?}", p, e); }
        }
    }
    Ok(())
}

fn load_history() -> Vec<HistoryEntry> {
    fs::read_to_string(HISTORY_FILE).ok().and_then(|d| serde_json::from_str(&d).ok()).unwrap_or_default()
}

fn save_history(h: &[HistoryEntry]) {
    let s = if h.len() > 30 { h.len() - 30 } else { 0 };
    if let Ok(d) = serde_json::to_string_pretty(&h[s..]) { let _ = fs::write(HISTORY_FILE, d); }
}
