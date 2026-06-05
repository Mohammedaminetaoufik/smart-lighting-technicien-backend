# Smart Lighting Technicien — API Test Guide

Backend port : **8081**  
Auth : **désactivée** (`AUTH_ENABLED=false`) — toutes les routes sont ouvertes pour les tests.

## Démarrage

```bash
cd smart-lighting-technicien/backend
cp .env.example .env
# Éditer .env avec vos credentials PostgreSQL
go run .
```

---

## Phase 1 — Santé & Contexte de test

### Health check
```bash
curl http://localhost:8081/api/mobile/health
```

### Contexte technicien (qui suis-je ?)
```bash
# Via query param
curl "http://localhost:8081/api/mobile/test-context?technician_id=1"

# Via header
curl -H "X-Test-Technician-Id: 2" http://localhost:8081/api/mobile/test-context

# Via défaut .env
curl http://localhost:8081/api/mobile/test-context
```

---

## Phase 2 — Dashboard technicien

```bash
curl "http://localhost:8081/api/mobile/me/dashboard?technician_id=1"
```

---

## Phase 3 — Bons de travail

### Liste des interventions
```bash
curl "http://localhost:8081/api/mobile/me/workorders?technician_id=1"

# Avec filtres
curl "http://localhost:8081/api/mobile/me/workorders?technician_id=1&status=in_progress&priority=high"
curl "http://localhost:8081/api/mobile/me/workorders?technician_id=1&zone=Agdal&limit=10"
```

### Détail d'un bon de travail
```bash
curl "http://localhost:8081/api/mobile/workorders/1?technician_id=1"
```

---

## Phase 4 — Actions intervention

### Accepter un bon de travail
```bash
curl -X POST http://localhost:8081/api/mobile/workorders/1/accept \
  -H "Content-Type: application/json" \
  -d '{"technician_id": 1}'

# Via query param
curl -X POST "http://localhost:8081/api/mobile/workorders/1/accept?technician_id=1"
```

### Démarrer une intervention
```bash
curl -X POST http://localhost:8081/api/mobile/workorders/1/start \
  -H "Content-Type: application/json" \
  -d '{"technician_id": 1}'
```

### Ajouter une note
```bash
curl -X POST http://localhost:8081/api/mobile/workorders/1/add-note \
  -H "Content-Type: application/json" \
  -d '{
    "technician_id": 1,
    "note": "Driver vérifié, problème au niveau de la connectique."
  }'
```

### Résoudre une intervention
```bash
curl -X POST http://localhost:8081/api/mobile/workorders/1/resolve \
  -H "Content-Type: application/json" \
  -d '{
    "technician_id": 1,
    "resolution_note": "Connectique réparée, lampadaire de nouveau opérationnel."
  }'
```

### Bloquer une intervention
```bash
curl -X POST http://localhost:8081/api/mobile/workorders/1/block \
  -H "Content-Type: application/json" \
  -d '{
    "technician_id": 1,
    "note": "Pièce de remplacement manquante — commande en cours."
  }'
```

---

## Phase 5 — Diagnostic lampadaire

### Diagnostic complet
```bash
curl "http://localhost:8081/api/mobile/lampadaires/1/diagnostic"
```

### Dernière télémétrie
```bash
curl "http://localhost:8081/api/mobile/lampadaires/1/telemetry/latest"
```

---

## Phase 6 — Carte

### Vue d'ensemble carte
```bash
curl http://localhost:8081/api/map/overview
```

### Lampadaires carte
```bash
curl "http://localhost:8081/api/map/lampadaires"
curl "http://localhost:8081/api/map/lampadaires?zone=Agdal&etat=offline"
curl "http://localhost:8081/api/map/lampadaires?lcu_id=1"
```

### LCUs carte
```bash
curl http://localhost:8081/api/map/lcus
```

### Connexions LCU → lampadaires
```bash
curl http://localhost:8081/api/map/connections
curl "http://localhost:8081/api/map/connections?lcu_id=1"
```

### Lampadaires sans localisation
```bash
curl http://localhost:8081/api/map/lampadaires/missing-location
```

### Contexte technicien sur carte (avec position GPS)
```bash
curl "http://localhost:8081/api/map/technician-context?technician_id=1&include_lcus=true&include_connections=true"

# Avec position GPS (recherche lamps à proximité dans 2km)
curl "http://localhost:8081/api/map/technician-context?technician_id=1&latitude=33.9911&longitude=-6.8494&radius=2000&include_lcus=true&include_connections=true"
```

### Mettre à jour la localisation GPS d'un lampadaire
```bash
curl -X POST http://localhost:8081/api/map/lampadaires/1/location \
  -H "Content-Type: application/json" \
  -d '{
    "latitude": 33.9911,
    "longitude": -6.8494,
    "accuracy": 8.5,
    "source": "technician_mobile"
  }'
```

---

## Phase 7 — Synchronisation JSON offline-first

### Bootstrap (données initiales pour le mode offline)
```bash
curl "http://localhost:8081/api/mobile/sync/bootstrap?technician_id=1"
```

### Pull (nouveautés depuis une date)
```bash
curl "http://localhost:8081/api/mobile/sync/pull?technician_id=1&since=2026-06-01T00:00:00Z"
```

### Push (envoyer les actions effectuées offline)
```bash
curl -X POST http://localhost:8081/api/mobile/sync/push \
  -H "Content-Type: application/json" \
  -d '{
    "technician_id": 1,
    "device_id": "test-phone-001",
    "device_name": "Phone Test",
    "platform": "android",
    "actions": [
      {
        "local_id": "test_note_001",
        "type": "ADD_NOTE",
        "entity": "work_order",
        "entity_id": 1,
        "payload": { "note": "Test note depuis sync JSON" },
        "created_at": "2026-06-04T15:10:00Z"
      }
    ]
  }'
```

### Push avec plusieurs types d'actions
```bash
curl -X POST http://localhost:8081/api/mobile/sync/push \
  -H "Content-Type: application/json" \
  -d '{
    "technician_id": 1,
    "device_id": "test-phone-001",
    "actions": [
      {
        "local_id": "accept_wo5_001",
        "type": "ACCEPT_WORK_ORDER",
        "entity": "work_order",
        "entity_id": 5,
        "payload": {},
        "created_at": "2026-06-04T10:00:00Z"
      },
      {
        "local_id": "note_wo5_002",
        "type": "ADD_NOTE",
        "entity": "work_order",
        "entity_id": 5,
        "payload": { "note": "Arrivé sur site, lampadaire inspecté." },
        "created_at": "2026-06-04T10:30:00Z"
      },
      {
        "local_id": "loc_lamp3_001",
        "type": "UPDATE_LOCATION",
        "entity": "lampadaire",
        "entity_id": 3,
        "payload": { "latitude": 33.9911, "longitude": -6.8494, "source": "gps" },
        "created_at": "2026-06-04T10:45:00Z"
      }
    ]
  }'
```

### Idempotence — envoyer 2× le même local_id
```bash
# Le même local_id retourne le résultat déjà connu sans ré-exécuter l'action
curl -X POST http://localhost:8081/api/mobile/sync/push \
  -H "Content-Type: application/json" \
  -d '{
    "technician_id": 1,
    "device_id": "test-phone-001",
    "actions": [
      {
        "local_id": "test_note_001",
        "type": "ADD_NOTE",
        "entity": "work_order",
        "entity_id": 1,
        "payload": { "note": "Doublon" },
        "created_at": "2026-06-04T15:10:00Z"
      }
    ]
  }'
```

### Full sync (push + bootstrap en une requête)
```bash
curl -X POST http://localhost:8081/api/mobile/sync/full \
  -H "Content-Type: application/json" \
  -d '{
    "technician_id": 1,
    "device_id": "test-phone-001",
    "actions": []
  }'
```

---

## Types d'actions supportées (sync push)

| Type | Entity | Payload requis |
|---|---|---|
| `ACCEPT_WORK_ORDER` | work_order | `{}` |
| `START_WORK_ORDER` | work_order | `{}` |
| `ADD_NOTE` | work_order | `{"note": "..."}` |
| `RESOLVE_WORK_ORDER` | work_order | `{"resolution_note": "..."}` |
| `BLOCK_WORK_ORDER` | work_order | `{"note": "..."}` |
| `UPDATE_LOCATION` | lampadaire | `{"latitude": 33.99, "longitude": -6.84, "source": "gps"}` |

---

## Workflow de test complet

```bash
# 1. Vérifier le serveur
curl http://localhost:8081/api/mobile/health

# 2. Voir mes interventions
curl "http://localhost:8081/api/mobile/me/workorders?technician_id=1"

# 3. Prendre le WO id=1 et l'accepter
curl -X POST "http://localhost:8081/api/mobile/workorders/1/accept?technician_id=1" \
  -H "Content-Type: application/json" -d '{}'

# 4. Démarrer
curl -X POST "http://localhost:8081/api/mobile/workorders/1/start?technician_id=1" \
  -H "Content-Type: application/json" -d '{}'

# 5. Consulter le diagnostic du lampadaire lié
curl http://localhost:8081/api/mobile/lampadaires/1/diagnostic

# 6. Ajouter une note
curl -X POST "http://localhost:8081/api/mobile/workorders/1/add-note" \
  -H "Content-Type: application/json" \
  -d '{"technician_id":1,"note":"Défaut identifié: fusible grillé."}'

# 7. Résoudre
curl -X POST "http://localhost:8081/api/mobile/workorders/1/resolve" \
  -H "Content-Type: application/json" \
  -d '{"technician_id":1,"resolution_note":"Fusible remplacé, lampadaire opérationnel."}'

# 8. Vérifier dans la plateforme web (port 8080) que le statut a changé
curl "http://localhost:8080/api/workorders/1"
```

---

## Notes de développement

- `DEFAULT_TECHNICIAN_ID` dans `.env` = technicien utilisé par défaut si aucun paramètre fourni
- Toutes les actions sont enregistrées dans `access_logs` (visible dans la plateforme web admin)
- Les `local_id` sont uniques — renvoyer le même `local_id` retourne le résultat déjà connu (idempotence)
- Les tables `mobile_devices` et `mobile_sync_logs` sont créées automatiquement au démarrage
