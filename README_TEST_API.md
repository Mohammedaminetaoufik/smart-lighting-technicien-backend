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
- Les tables `mobile_devices`, `mobile_sync_logs` et `field_notes` sont créées automatiquement au démarrage

---

# PHASE 2 — Endpoints terrain complets

## Dashboard enrichi

```bash
curl "http://localhost:8081/api/mobile/me/dashboard?technician_id=1"
# Retourne: sync, stats{assigned,urgent,in_progress,completed_today},
#           next_work_order, map_summary, important_alerts
```

## Lampadaires (consultation terrain)

```bash
# Liste avec filtres
curl "http://localhost:8081/api/mobile/lampadaires"
curl "http://localhost:8081/api/mobile/lampadaires?etat=offline&zone=Agdal&search=LP-04"

# Détail complet (caractéristiques + télémétrie + alertes + interventions liées)
curl "http://localhost:8081/api/mobile/lampadaires/1/details?technician_id=1"

# Sous-ressources
curl "http://localhost:8081/api/mobile/lampadaires/1/diagnostic"
curl "http://localhost:8081/api/mobile/lampadaires/1/telemetry/latest"
curl "http://localhost:8081/api/mobile/lampadaires/1/alerts"
curl "http://localhost:8081/api/mobile/lampadaires/1/workorders"

# Note terrain
curl -X POST http://localhost:8081/api/mobile/lampadaires/1/field-note \
  -H "Content-Type: application/json" \
  -d '{"technician_id":1,"note":"Poteau légèrement penché, à surveiller."}'

# Mise à jour GPS (autorisée si lié à intervention ou en mise en service)
curl -X POST http://localhost:8081/api/mobile/lampadaires/1/location \
  -H "Content-Type: application/json" \
  -d '{"technician_id":1,"latitude":33.9911,"longitude":-6.8494,"accuracy":8,"source":"technician_mobile"}'
```

## LCUs (consultation + diagnostic terrain)

```bash
# Liste avec compteurs de lampadaires
curl "http://localhost:8081/api/mobile/lcus"

# Détail + lampadaires rattachés
curl "http://localhost:8081/api/mobile/lcus/1/details"
curl "http://localhost:8081/api/mobile/lcus/1/lampadaires"
curl "http://localhost:8081/api/mobile/lcus/1/diagnostic"

# Test de connectivité (simulation)
curl -X POST http://localhost:8081/api/mobile/lcus/1/test \
  -H "Content-Type: application/json" -d '{"technician_id":1}'

# Synchronisation LCU (simulation)
curl -X POST http://localhost:8081/api/mobile/lcus/1/sync \
  -H "Content-Type: application/json" -d '{"technician_id":1}'

# Note terrain LCU
curl -X POST http://localhost:8081/api/mobile/lcus/1/field-note \
  -H "Content-Type: application/json" \
  -d '{"technician_id":1,"note":"Armoire LCU accessible, antenne OK."}'
```

## Mise en service (commissioning)

```bash
# Liste des lampadaires à mettre en service
curl "http://localhost:8081/api/mobile/commissioning"
curl "http://localhost:8081/api/mobile/commissioning?status=located"

# Détail d'une tâche
curl "http://localhost:8081/api/mobile/commissioning/1"

# Confirmer/corriger GPS (passe en 'located')
curl -X POST http://localhost:8081/api/mobile/commissioning/1/update-gps \
  -H "Content-Type: application/json" \
  -d '{"technician_id":1,"latitude":33.9911,"longitude":-6.8494}'

# Tester la communication
curl -X POST http://localhost:8081/api/mobile/commissioning/1/test-communication \
  -H "Content-Type: application/json" -d '{"technician_id":1}'

# Tester le dimming (passe en 'tested')
curl -X POST http://localhost:8081/api/mobile/commissioning/1/test-dimming \
  -H "Content-Type: application/json" -d '{"technician_id":1}'

# Valider la mise en service (passe en 'commissioned')
curl -X POST http://localhost:8081/api/mobile/commissioning/1/validate \
  -H "Content-Type: application/json" -d '{"technician_id":1}'

# Signaler un échec
curl -X POST http://localhost:8081/api/mobile/commissioning/1/fail \
  -H "Content-Type: application/json" \
  -d '{"technician_id":1,"reason":"Driver défectueux, remplacement nécessaire."}'

# Ajouter une note de mise en service
curl -X POST http://localhost:8081/api/mobile/commissioning/1/add-note \
  -H "Content-Type: application/json" \
  -d '{"technician_id":1,"note":"Configuration validée sur site."}'
```

## Carte enrichie

```bash
# Overview complet (lampadaires + lcus + connections + stats + center)
curl "http://localhost:8081/api/map/overview"

# Contexte technicien (assignés, proximité, lcus, connexions, missing)
curl "http://localhost:8081/api/map/technician-context?technician_id=1&include_lcus=true&include_connections=true"
```

## Bootstrap offline enrichi

```bash
# Retourne: dashboard + work_orders + lampadaires + lcus + connections
#           + commissioning + missing_location + last_sync_at
curl "http://localhost:8081/api/mobile/sync/bootstrap?technician_id=1"
```

## Nouveaux types d'actions sync (push offline)

```bash
curl -X POST http://localhost:8081/api/mobile/sync/push \
  -H "Content-Type: application/json" \
  -d '{
    "technician_id": 1,
    "device_id": "test-phone-001",
    "actions": [
      { "local_id": "lamp_note_001", "type": "ADD_LAMPADAIRE_FIELD_NOTE", "entity": "lampadaire", "entity_id": 1, "payload": {"note":"Note offline lampadaire"}, "created_at": "2026-06-05T10:00:00Z" },
      { "local_id": "lcu_note_001",  "type": "ADD_LCU_FIELD_NOTE", "entity": "lcu", "entity_id": 1, "payload": {"note":"Note offline LCU"}, "created_at": "2026-06-05T10:01:00Z" },
      { "local_id": "lcu_test_001",  "type": "TEST_LCU_CONNECTIVITY", "entity": "lcu", "entity_id": 1, "payload": {}, "created_at": "2026-06-05T10:02:00Z" },
      { "local_id": "comm_gps_001",  "type": "COMMISSIONING_UPDATE_GPS", "entity": "commissioning", "entity_id": 1, "payload": {"latitude":33.99,"longitude":-6.84}, "created_at": "2026-06-05T10:03:00Z" },
      { "local_id": "comm_val_001",  "type": "COMMISSIONING_VALIDATE", "entity": "commissioning", "entity_id": 1, "payload": {}, "created_at": "2026-06-05T10:04:00Z" }
    ]
  }'
```

**Types d'actions supportés** : `ACCEPT_WORK_ORDER`, `START_WORK_ORDER`, `ADD_NOTE`, `RESOLVE_WORK_ORDER`, `BLOCK_WORK_ORDER`, `UPDATE_LOCATION`, `ADD_LAMPADAIRE_FIELD_NOTE`, `ADD_LCU_FIELD_NOTE`, `TEST_LCU_CONNECTIVITY`, `SYNC_LCU`, `COMMISSIONING_UPDATE_GPS`, `COMMISSIONING_TEST_COMMUNICATION`, `COMMISSIONING_TEST_DIMMING`, `COMMISSIONING_VALIDATE`, `COMMISSIONING_FAIL`, `COMMISSIONING_ADD_NOTE`

---

# Scénario de collaboration Web ↔ Mobile (Phase 2)

```bash
# 1. Admin web assigne une intervention au technicien 1 (via /workorders dans la webapp)

# 2. Mobile voit l'intervention
curl "http://localhost:8081/api/mobile/me/workorders?technician_id=1"

# 3. Mobile accepte → Web admin voit status 'accepted' (même DB)
curl -X POST "http://localhost:8081/api/mobile/workorders/15/accept" \
  -H "Content-Type: application/json" -d '{"technician_id":1}'

# 4. Mobile démarre → Web admin voit 'in_progress'
curl -X POST "http://localhost:8081/api/mobile/workorders/15/start" \
  -H "Content-Type: application/json" -d '{"technician_id":1}'

# 5. Mobile consulte le lampadaire et son diagnostic
curl "http://localhost:8081/api/mobile/lampadaires/44/details"

# 6. Mobile ajoute une note → visible dans work_order_logs côté admin
curl -X POST "http://localhost:8081/api/mobile/workorders/15/add-note" \
  -H "Content-Type: application/json" -d '{"technician_id":1,"note":"Fusible grillé identifié."}'

# 7. Mobile teste la LCU
curl -X POST "http://localhost:8081/api/mobile/lcus/3/test" \
  -H "Content-Type: application/json" -d '{"technician_id":1}'

# 8. Mobile résout → Web admin voit 'resolved' + resolved_at
curl -X POST "http://localhost:8081/api/mobile/workorders/15/resolve" \
  -H "Content-Type: application/json" -d '{"technician_id":1,"resolution_note":"Fusible remplacé."}'

# 9. Vérifier côté web admin (port 8080)
curl "http://localhost:8080/api/workorders/15"
```
