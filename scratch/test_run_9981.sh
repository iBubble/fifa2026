#!/bin/bash
docker logs FIFA2026 2>&1 | grep -E "Predict|Refine|Ollama" | tail -n 25
echo "Check executed at: $(date) (Random: 9981)"
