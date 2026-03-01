/**
 * Gemini Model Lister
 * Fetches all available models for your API key from the v1beta endpoint.
 */

const API_KEY = process.env.GEMINI_API_KEY;

if (!API_KEY) {
  console.error("\x1b[31mError: GEMINI_API_KEY is not set.\x1b[0m");
  process.exit(1);
}

async function listAvailableModels() {
  const url = `https://generativelanguage.googleapis.com/v1beta/models?key=${API_KEY}`;

  console.log("\x1b[33mFetching available Gemini models...\x1b[0m\n");

  try {
    const response = await fetch(url);
    const data = await response.json();

    if (data.error) {
      console.error("\x1b[31mAPI Error:\x1b[0m", data.error.message);
      return;
    }

    // Filter for models that actually support content generation
    const genModels = data.models.filter(m => 
      m.supportedGenerationMethods.includes("generateContent")
    );

    console.log("ID".padEnd(35) + "DISPLAY NAME");
    console.log("-".repeat(60));

    genModels.forEach(model => {
      // Clean up the name (remove 'models/' prefix)
      const shortId = model.name.replace("models/", "");
      console.log(`\x1b[32m${shortId.padEnd(35)}\x1b[0m ${model.displayName}`);
    });

    console.log("\n\x1b[34mUsage:\x1b[0m Update MODEL_NAME in your script with one of the IDs above.");

  } catch (error) {
    console.error("\x1b[31mFetch Error:\x1b[0m", error.message);
  }
}

listAvailableModels();
