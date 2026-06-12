package api

const openAPIJSON = `{
  "openapi": "3.1.0",
  "info": {
    "title": "Lead Scout API",
    "version": "0.1.0",
    "description": "Local API for collecting, scoring, reviewing, and testing Lead Scout leads."
  },
  "servers": [
    {
      "url": "http://localhost:8080",
      "description": "Local development server"
    }
  ],
  "paths": {
    "/health": {
      "get": {
        "summary": "Health check",
        "responses": {
          "200": {
            "description": "Server is healthy",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/HealthResponse" }
              }
            }
          }
        }
      }
    },
    "/api/collect": {
      "post": {
        "summary": "Run collectors",
        "description": "Collect from one source or all configured sources, store raw items, normalize leads, and dedupe.",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": { "$ref": "#/components/schemas/CollectRequest" },
              "examples": {
                "hn": { "value": { "source": "hn" } },
                "all": { "value": { "all": true } }
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": "Collection summary",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/CollectResponse" }
              }
            }
          }
        }
      }
    },
    "/api/score": {
      "post": {
        "summary": "Score pending leads",
        "requestBody": {
          "required": false,
          "content": {
            "application/json": {
              "schema": { "$ref": "#/components/schemas/ScoreRequest" },
              "examples": {
                "last24h": { "value": { "since": "24h" } }
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": "Scoring summary",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/ScoreResponse" }
              }
            }
          }
        }
      }
    },
    "/api/digest": {
      "post": {
        "summary": "Preview or send founder digest",
        "description": "By default this previews digest candidates. Set send=true to send to Telegram.",
        "requestBody": {
          "required": false,
          "content": {
            "application/json": {
              "schema": { "$ref": "#/components/schemas/DigestRequest" }
            }
          }
        },
        "responses": {
          "200": {
            "description": "Digest response",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/DigestResponse" }
              }
            }
          }
        }
      }
    },
    "/api/telegram/test": {
      "post": {
        "summary": "Send Telegram test message",
        "description": "Sends a small test message to the configured TELEGRAM_CHAT_ID. Use this to verify bot token and chat ID from Scalar.",
        "responses": {
          "200": {
            "description": "Telegram test message sent",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "sent": { "type": "boolean" }
                  },
                  "required": ["sent"]
                }
              }
            }
          }
        }
      }
    },
    "/api/telegram/lead-test": {
      "post": {
        "summary": "Send custom Telegram lead test",
        "description": "Sends one pasted lead+score payload to Telegram without storing it. This is for testing Scalar and Telegram formatting.",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": { "$ref": "#/components/schemas/LeadWithScore" },
              "examples": {
                "upworkGig": {
                  "value": {
                    "lead": {
                      "id": 1042,
                      "source": "Upwork",
                      "category": "gig",
                      "title": "React/TypeScript Developer Needed for Dashboard Overhaul",
                      "body": "We are looking for an experienced frontend developer to refactor our legacy data dashboard.",
                      "url": "https://www.upwork.com/jobs/~01abc123xyz",
                      "author": "Sarah Jenkins",
                      "company": "Apex Analytics Corp",
                      "location": "Remote (US/Europe)",
                      "compensation": "$4,500 fixed price",
                      "state": "new"
                    },
                    "score": {
                      "score": 92,
                      "rationale": "Strong short-term dashboard contract fit.",
                      "draft_opener": "Hi Sarah, I saw your dashboard overhaul post...",
                      "should_notify": true,
                      "model": "manual-test",
                      "prompt_version": "manual-test"
                    }
                  }
                }
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": "Telegram lead test sent",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "sent": { "type": "boolean" }
                  },
                  "required": ["sent"]
                }
              }
            }
          }
        }
      }
    },
    "/api/leads": {
      "get": {
        "summary": "List leads",
        "parameters": [
          { "name": "category", "in": "query", "schema": { "$ref": "#/components/schemas/Category" } },
          { "name": "state", "in": "query", "schema": { "$ref": "#/components/schemas/LeadState" } },
          { "name": "limit", "in": "query", "schema": { "type": "integer", "minimum": 1, "maximum": 100, "default": 50 } }
        ],
        "responses": {
          "200": {
            "description": "Lead list",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/LeadListResponse" }
              }
            }
          }
        }
      }
    },
    "/api/leads/{id}/state": {
      "patch": {
        "summary": "Update lead state",
        "parameters": [
          { "name": "id", "in": "path", "required": true, "schema": { "type": "integer", "format": "int64" } }
        ],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": { "$ref": "#/components/schemas/UpdateStateRequest" },
              "examples": {
                "save": { "value": { "state": "saved", "note": "Reviewed in Scalar docs" } }
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": "Updated state",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/UpdateStateResponse" }
              }
            }
          }
        }
      }
    }
  },
  "components": {
    "schemas": {
      "Category": {
        "type": "string",
        "enum": ["gig", "founder", "job"]
      },
      "LeadState": {
        "type": "string",
        "enum": ["new", "saved", "rejected", "approached", "replied", "call", "won", "lost"]
      },
      "HealthResponse": {
        "type": "object",
        "properties": {
          "ok": { "type": "boolean" },
          "service": { "type": "string" }
        },
        "required": ["ok", "service"]
      },
      "CollectRequest": {
        "type": "object",
        "properties": {
          "source": { "type": "string", "enum": ["hn", "braintrust", "remoteok", "wwr", "reddit"] },
          "all": { "type": "boolean", "default": false }
        }
      },
      "CollectResponse": {
        "type": "object",
        "properties": {
          "sources": { "type": "array", "items": { "$ref": "#/components/schemas/SourceCollectResult" } }
        },
        "required": ["sources"]
      },
      "SourceCollectResult": {
        "type": "object",
        "properties": {
          "source": { "type": "string" },
          "fetched": { "type": "integer" },
          "normalized": { "type": "integer" },
          "skipped": { "type": "boolean" },
          "error": { "type": "string" }
        },
        "required": ["source", "fetched", "normalized", "skipped"]
      },
      "ScoreRequest": {
        "type": "object",
        "properties": {
          "since": { "type": "string", "default": "24h" }
        }
      },
      "ScoreResponse": {
        "type": "object",
        "properties": {
          "scored": { "type": "integer" }
        },
        "required": ["scored"]
      },
      "DigestRequest": {
        "type": "object",
        "properties": {
          "send": { "type": "boolean", "default": false },
          "limit": { "type": "integer", "default": 20, "minimum": 1, "maximum": 100 }
        }
      },
      "DigestResponse": {
        "type": "object",
        "properties": {
          "sent": { "type": "boolean" },
          "message": { "type": "string" },
          "leads": { "type": "array", "items": { "$ref": "#/components/schemas/LeadWithScore" } }
        },
        "required": ["sent", "leads"]
      },
      "LeadListResponse": {
        "type": "object",
        "properties": {
          "leads": { "type": "array", "items": { "$ref": "#/components/schemas/LeadWithScore" } }
        },
        "required": ["leads"]
      },
      "LeadWithScore": {
        "type": "object",
        "properties": {
          "lead": { "$ref": "#/components/schemas/Lead" },
          "score": { "$ref": "#/components/schemas/LeadScore" }
        },
        "required": ["lead", "score"]
      },
      "Lead": {
        "type": "object",
        "properties": {
          "id": { "type": "integer", "format": "int64" },
          "source": { "type": "string" },
          "category": { "$ref": "#/components/schemas/Category" },
          "title": { "type": "string" },
          "body": { "type": "string" },
          "url": { "type": "string" },
          "author": { "type": "string" },
          "company": { "type": "string" },
          "location": { "type": "string" },
          "compensation": { "type": "string" },
          "state": { "$ref": "#/components/schemas/LeadState" },
          "created_at": { "type": "string", "format": "date-time" }
        },
        "required": ["id", "source", "category", "title", "url", "state", "created_at"]
      },
      "LeadScore": {
        "type": "object",
        "properties": {
          "score": { "type": "integer" },
          "rationale": { "type": "string" },
          "draft_opener": { "type": "string" },
          "should_notify": { "type": "boolean" },
          "model": { "type": "string" },
          "prompt_version": { "type": "string" }
        },
        "required": ["score", "rationale", "should_notify", "model", "prompt_version"]
      },
      "UpdateStateRequest": {
        "type": "object",
        "properties": {
          "state": { "$ref": "#/components/schemas/LeadState" },
          "note": { "type": "string" }
        },
        "required": ["state"]
      },
      "UpdateStateResponse": {
        "type": "object",
        "properties": {
          "id": { "type": "integer", "format": "int64" },
          "state": { "$ref": "#/components/schemas/LeadState" },
          "ok": { "type": "boolean" }
        },
        "required": ["id", "state", "ok"]
      }
    }
  }
}`
