# Gemini API Module

This module provides integration with Google's Gemini AI API for chat and log analysis functionality.

## Overview

The Gemini module offers two main endpoints:
1. **Chat Handler** (`/api/gemini/chat`) - Simple chat interface with Gemini
2. **Analyze Log Handler** (`/api/gemini/analyze`) - Two-stage log analysis system with triage and expert analysis

## API Endpoints

### 1. Chat Endpoint

**Endpoint:** `POST /api/gemini/chat`

Simple chat interface that accepts a prompt and optionally a file.

#### JSON Request Format

```bash
curl -X POST http://localhost:8080/api/gemini/chat \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "<your prompt here>"
  }'
```

#### Multipart Form Request Format

```bash
curl -X POST http://localhost:8080/api/gemini/chat \
  -F "prompt=Your prompt text here" \
  -F "file=@logfile.txt"
```

**Request Fields:**
- `prompt` (optional): Text prompt to send to Gemini
- `file` (optional): Text file to upload (max 1MB)

**Response:**
```json
{
  "text": "Gemini's response text"
}
```

---

### 2. Analyze Log Endpoint

**Endpoint:** `POST /api/gemini/analyze`

Two-stage log analysis system that:
1. **Stage 1 (Triage)**: Classifies the log into one of three analysis modes
2. **Stage 2 (Expert)**: Performs detailed analysis using the appropriate expert prompt

#### Request Format

**JSON Body:**
```bash
curl -X POST http://localhost:8080/api/gemini/analyze \
  -H "Content-Type: application/json" \
  -d '{
    "triage_prompt": "# Role\nYou are a Lead System Architect...",
    "expert_prompts": {
      "MODE_CLIENT_CONFIG": "# Role\nYou are a DevOps & API Specialist...",
      "MODE_DATABASE_LOGIC": "# Role\nYou are a Senior Backend Developer...",
      "MODE_PERFORMANCE_CONCURRENCY": "# Role\nYou are a Principal SRE..."
    },
    "file_content": "...log file content..."
  }'
```

**Multipart Form:**
```bash
curl -X POST http://localhost:8080/api/gemini/analyze \
  -F "triage_prompt=# Role\nYou are a Lead System Architect..." \
  -F 'expert_prompts={"MODE_CLIENT_CONFIG":"...","MODE_DATABASE_LOGIC":"...","MODE_PERFORMANCE_CONCURRENCY":"..."}' \
  -F "file=@logfile.txt"
```

**Request Fields:**
- `triage_prompt` (required): Prompt for Stage 1 triage classification
- `expert_prompts` (required): Map of expert prompts keyed by analysis mode
  - Keys must be: `MODE_CLIENT_CONFIG`, `MODE_DATABASE_LOGIC`, `MODE_PERFORMANCE_CONCURRENCY`
- `file_content` (required): Log file content (or use `file` field in multipart)

**Response:**
```json
{
  "triage": {
    "analysis_mode": "MODE_CLIENT_CONFIG",
    "detected_keywords": ["405 Method Not Allowed", "Bad Request"],
    "primary_error_log": "Error: 405 Method Not Allowed"
  },
  "expert_analysis": "## Root Cause\n- **Type:** HTTP Method Mismatch\n..."
}
```

---

## Prompts Reference

Ready-to-use prompts for both endpoints are available in [`prompt.md`](./prompt.md):

- **Chat Endpoint Prompt**: System and user prompts for crash reproduction
- **Triage Prompt**: Classification prompt for Stage 1 analysis
- **Expert Prompts**: Three expert prompts for Stage 2 analysis

You can copy these prompts directly from the markdown file and use them in your API requests.