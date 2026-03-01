/**
 * Gemini CLI - Version 4.1 "The Vacuum"
 * - Fixed: All empty lines are now truly empty (0 chars).
 * - Fixed: No trailing spaces in config or logic.
 * - Success: Unicode/Emoji support is stable.
 */

const { GoogleGenerativeAI, HarmCategory, HarmBlockThreshold } = require("@google/generative-ai");
const readline = require("readline");
const fs = require("fs");

const API_KEY = process.env.GEMINI_API_KEY;
const MODEL_NAME = "gemini-3-flash-preview";
const HISTORY_FILE = "/app/chat_history.json";
const MAX_WIDTH = 100;
const OFFSET_FACTOR = 0.20;
const MAX_HISTORY_LENGTH = 20;

const genAI = new GoogleGenerativeAI(API_KEY);
const model = genAI.getGenerativeModel({
  model: MODEL_NAME,
  systemInstruction: {
    role: "system",
    parts: [{
      text: "You are a helpful, professional, and grounded AI assistant. " +
            "Your goal is to provide clear, concise, and accurate answers. " +
            "Always respond in the language used by the user. " +
            "Maintain a friendly yet focused tone, and use formatting like bold text " +
            "to make information easy to digest."
    }]
  }
});

let localHistory = [];

function getPadding() {
  const termWidth = process.stdout.columns || 120;
  const availableSpace = termWidth - MAX_WIDTH;
  return " ".repeat(Math.max(0, Math.floor(availableSpace * OFFSET_FACTOR)));
}

function renderFormatted(text, isDimmed = false, skipFirstPadding = false) {
  const padding = getPadding();
  let currentColumn = skipFirstPadding ? (padding.length + 8) : 0;
  let lastChar = "";
  let isBold = false;

  if (isDimmed) process.stdout.write("\x1b[2m");
  if (!skipFirstPadding) process.stdout.write(padding);

  for (const char of text) {
    if (char === '*' && lastChar === '*') {
      isBold = !isBold;
      process.stdout.write(isBold ? '\x1b[1m' : (isDimmed ? '\x1b[22m\x1b[2m' : '\x1b[22m'));
      lastChar = ""; continue;
    }
    if (char === '*') { lastChar = '*'; continue; }
    if (lastChar === '*') { process.stdout.write('*'); lastChar = ""; currentColumn++; }

    if (char === '\n') {
      process.stdout.write('\n' + padding);
      currentColumn = 0;
    } else {
      process.stdout.write(char);
      currentColumn++;
      if (currentColumn >= MAX_WIDTH && (char === ' ' || char === '\t')) {
        process.stdout.write('\n' + padding);
        currentColumn = 0;
      }
    }
    lastChar = char;
  }
  process.stdout.write("\x1b[0m");
}

function displayHistory(history) {
  if (history.length === 0) return;
  const padding = getPadding();
  console.log("\n" + padding + "\x1b[2m\x1b[33m--- Previous Conversation ---\x1b[0m");
  history.forEach(entry => {
    const roleColor = entry.role === "user" ? "\x1b[36m" : "\x1b[35m";
    process.stdout.write(padding + `${roleColor}${entry.role === "user" ? "You" : "Gemini"}:\x1b[0m `);
    renderFormatted(entry.parts[0].text, true, true);
    process.stdout.write("\n\n");
  });
  console.log(padding + "\x1b[2m\x1b[33m------------------------------\x1b[0m\n");
}

async function handleStream(chatSession, userInput) {
  const padding = getPadding();
  let fullModelResponse = "";
  let firstTokenReceived = false;
  localHistory.push({ role: "user", parts: [{ text: userInput }] });
  commitHistoryToDisk();
  process.stdout.write("\n" + padding + "\x1b[2m\x1b[35m[Thinking...]\x1b[0m");
  try {
    const result = await chatSession.sendMessageStream(userInput);
    for await (const chunk of result.stream) {
      const text = chunk.text();
      fullModelResponse += text;
      if (!firstTokenReceived) {
        readline.clearLine(process.stdout, 0);
        readline.cursorTo(process.stdout, 0);
        process.stdout.write(padding + "\x1b[35mGemini:\x1b[0m ");
        firstTokenReceived = true;
      }
      renderFormatted(text, false, true);
    }
    process.stdout.write('\n\n');
    localHistory.push({ role: "model", parts: [{ text: fullModelResponse }] });
    commitHistoryToDisk();
  } catch (error) {
    process.stdout.write("\n" + padding + "\x1b[31m[Error: " + error.message + "]\x1b[0m\n");
  }
}

async function run() {
  localHistory = loadHistory() || [];
  displayHistory(localHistory);
  let chatSession = model.startChat({ history: localHistory });
  const rl = readline.createInterface({ input: process.stdin, output: process.stdout, prompt: getPadding() + "\x1b[36mYou > \x1b[0m" });
  rl.on("SIGINT", () => { process.stdout.write("\n" + getPadding() + "\x1b[33m[System] Bye!\x1b[0m\n"); process.exit(0); });
  console.log(getPadding() + `\x1b[33m--- Gemini Pro Shell v4.1 (${MODEL_NAME}) ---\x1b[0m`);
  rl.prompt();
  rl.on("line", async (line) => {
    const input = line.trim();
    if (input.toLowerCase() === "exit") process.exit(0);
    if (input) await handleStream(chatSession, input);
    rl.prompt();
  });
}

function loadHistory() {
  if (fs.existsSync(HISTORY_FILE)) {
    try {
      const data = fs.readFileSync(HISTORY_FILE, "utf-8");
      let parsed = JSON.parse(data);
      let pruned = parsed.length > MAX_HISTORY_LENGTH ? parsed.slice(-MAX_HISTORY_LENGTH) : parsed;
      while (pruned.length > 0 && pruned[0].role !== 'user') pruned.shift();
      return pruned;
    } catch (e) { return []; }
  }
  return [];
}

function commitHistoryToDisk() {
  try { fs.writeFileSync(HISTORY_FILE, JSON.stringify(localHistory, null, 2)); } catch (err) {}
}

run();
