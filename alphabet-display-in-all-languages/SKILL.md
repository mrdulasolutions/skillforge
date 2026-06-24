---
name: alphabet-display-in-all-languages
description: "Displays the alphabet of any language when the user asks for it, Use when the user requests the alphabet in a specific language or any language."
---

# Alphabet Display In All Languages

## When to use this skill
Use this skill when a user wants to see the alphabet of a language, any language, or multiple languages without specifying which one.

## Inputs
- **language** (optional): The target language name or ISO code. If omitted, the skill returns the alphabet for all languages.

## Instructions
1. Receive the user request.
2. If a language is specified, validate it; otherwise, treat as “all”.
3. Look up the alphabet string for the requested language(s).
4. Return the alphabet in a clear format.

## Output
- The alphabet string for the requested language(s) in plain text.

## Examples
- User: “Show me the alphabet in Spanish.” → Output: “A B C ... Z”
- User: “Alphabet” → Output: list of alphabets for all languages, each labeled.
