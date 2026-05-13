/**
 * gemini.js - v4.5 (Final Polish)
 * - Restored: Thinking indicator (Purple)
 * - Fixed: Safe overwrite to preserve layout
 * - Added: Multi-line support via backslash (\)
 */

const { GoogleGenerativeAI } = require("@google/generative-ai");
const fs = require("fs");
const readline = require("readline");

const API_KEY = process.env.GEMINI_API_KEY;
const MODEL_NAME = "gemini-3-flash-preview";
const HISTORY_FILE = "chat_history.json";
const MAX_LINE_CHARS = 120;

if (!API_KEY) {
    console.error("\x1b[31mError: GEMINI_API_KEY not set.\x1b[0m");
    process.exit(1);
}

const genAI = new GoogleGenerativeAI(API_KEY);
const model = genAI.getGenerativeModel({
    model: MODEL_NAME,
    systemInstruction: "Helpful AI. Formatting: Use [31m Red, [32m Green, [33m Yellow, [34m Blue, [35m Magenta, [36m Cyan and Markdown."
});

const rl = readline.createInterface({
    input: process.stdin,
    output: process.stdout,
    terminal: true
});

let formatterState = {
    boldActive: false,
    italicActive: false,
    lineLength: 0,
    colorBuffer: "",
    isCollectingColor: false,
    markdownBuffer: ""
};

function getPadding() {
    const width = process.stdout.columns || 140;
    let p = Math.floor((width - MAX_LINE_CHARS - 10) * 0.1);
    return " ".repeat(Math.max(0, p));
}

function printFormatted(text, indent, state) {
    const chars = Array.from(text);
    for (let i = 0; i < chars.length; i++) {
        let char = chars[i];
        if (char === '\n') {
            process.stdout.write("\n" + indent);
            state.lineLength = 0;
            continue;
        }
        if (char === '[') {
            state.isCollectingColor = true;
            state.colorBuffer = "";
            continue;
        }
        if (state.isCollectingColor) {
            state.colorBuffer += char;
            if (char === 'm') {
                process.stdout.write("\x1b[" + state.colorBuffer);
                state.isCollectingColor = false;
                state.colorBuffer = "";
            }
            continue;
        }
        if (char === '*') {
            state.markdownBuffer += '*';
            if (state.markdownBuffer === '**') {
                process.stdout.write(state.boldActive ? "\x1b[22m" : "\x1b[1m");
                state.boldActive = !state.boldActive;
                state.markdownBuffer = "";
            } else {
                if (i + 1 >= chars.length || chars[i+1] !== '*') {
                    process.stdout.write(state.italicActive ? "\x1b[23m" : "\x1b[3m");
                    state.italicActive = !state.italicActive;
                    state.markdownBuffer = "";
                }
            }
            continue;
        }
        if (state.lineLength >= MAX_LINE_CHARS && (char === ' ' || char === '\t')) {
            process.stdout.write("\n" + indent);
            state.lineLength = 0;
        } else {
            process.stdout.write(char);
            state.lineLength++;
        }
    }
}

async function startChat() {
    let history = [];
    if (fs.existsSync(HISTORY_FILE)) {
        try { history = JSON.parse(fs.readFileSync(HISTORY_FILE, "utf-8")); } catch(e) {}
    }

    const p = getPadding();
    const indent = p + "        ";
    const promptString = `${p}\x1b[36mYou > \x1b[0m`;
    let inputBuffer = "";

    history.forEach(entry => {
        let hState = { boldActive: false, italicActive: false, lineLength: 0, colorBuffer: "", isCollectingColor: false, markdownBuffer: "" };
        if (entry.role === "user") {
            console.log(`\n${p}\x1b[36mYou > \x1b[0m${entry.parts[0].text}`);
        } else {
            process.stdout.write(`${p}\x1b[35mGemini:\x1b[0m `);
            printFormatted(entry.parts[0].text, indent, hState);
            process.stdout.write("\x1b[0m\n");
        }
    });

    console.log(`${p}\x1b[33m--- Gemini Pro Shell v4.5 (${MODEL_NAME}) ---\x1b[0m\n`);

    const chat = model.startChat({ history });

    const promptUser = () => {
        const currentPrompt = inputBuffer ? `${p}\x1b[33m    > \x1b[0m` : promptString;
        rl.setPrompt(currentPrompt);
        rl.prompt();

        rl.once('line', async (line) => {
            let trimmedLine = line.trim();

            if (trimmedLine.endsWith("\\")) {
                inputBuffer += trimmedLine.slice(0, -1) + " ";
                promptUser();
                return;
            }

            const input = inputBuffer + line;
            inputBuffer = ""; 

            if (input.toLowerCase() === "exit" || input.toLowerCase() === "quit") {
                process.exit(0);
            }

            // Zeige Thinking an
            process.stdout.write(`\n${p}\x1b[35m[Thinking...]\x1b[0m`);

            formatterState = { boldActive: false, italicActive: false, lineLength: 0, colorBuffer: "", isCollectingColor: false, markdownBuffer: "" };
            let fullResponse = "";

            try {
                const result = await chat.sendMessageStream(input);
                
                // Wir springen zum Anfang der Zeile (\r), schreiben das Padding p 
                // und löschen dann den Thinking-Text mit \x1b[K
                process.stdout.write(`\r${p}\x1b[K\x1b[35mGemini:\x1b[0m `);

                for await (const chunk of result.stream) {
                    const text = chunk.text();
                    fullResponse += text;
                    printFormatted(text, indent, formatterState);
                }
                process.stdout.write("\x1b[0m\n\n");

                history.push({ role: "user", parts: [{ text: input }] });
                history.push({ role: "model", parts: [{ text: fullResponse }] });
                fs.writeFileSync(HISTORY_FILE, JSON.stringify(history.slice(-20), null, 2));
            } catch (error) {
                console.error("\n\x1b[31mError:\x1b[0m", error.message);
            }
            promptUser();
        });
    };
    promptUser();
}

startChat();
