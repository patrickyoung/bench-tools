#!/usr/bin/env python3
"""Count exact duplicate signup lines from stdin without a model."""
import json
import sys

addresses = [line.strip() for line in sys.stdin if line.strip()]
print(json.dumps({"rows": len(addresses), "unique": len(set(addresses)),
                  "duplicates": len(addresses) - len(set(addresses))}))
