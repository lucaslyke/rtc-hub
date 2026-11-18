#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
mkdir -p .data
docker compose cp calc-regular:/app/calculadora/db/calculadora-pro.db .data/
