# Sur Windows, utiliser les commandes PowerShell :
# .\dev.ps1 up / down / logs / ps / clean

COMPOSE := docker compose

.PHONY: help up down logs ps config

help:
	@echo "  make up      - Démarrer les services"
	@echo "  make down    - Arrêter les services (volumes conservés)"
	@echo "  make logs    - Suivre les logs"
	@echo "  make ps      - État des services"
	@echo "  make config  - Valider docker-compose.yml avec .env"

up:
	$(COMPOSE) up -d

down:
	$(COMPOSE) down

logs:
	$(COMPOSE) logs -f

ps:
	$(COMPOSE) ps

config:
	$(COMPOSE) config --quiet
