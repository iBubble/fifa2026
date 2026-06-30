#!/bin/bash
docker logs FIFA2026 2>&1 | tail -n 25
echo "Check executed at: $(date) (Dynamic: 1782808620)"
