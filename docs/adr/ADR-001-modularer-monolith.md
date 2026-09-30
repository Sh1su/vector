# ADR-001 – Modularer Monolith mit fachlichen Modulen

- **Status:** akzeptiert · **Datum:** 2026-09-30

## Kontext
Der Auftrag gibt einen modularen Monolithen in Go vor (Abschnitt 4, 5.2). Das Ressourcenbudget liegt bei 1 vCPU / 1 GB RAM für den Basis-Stack, das Backend soll im Leerlauf unter 50 MB bleiben. In der Altanwendung war Fachlogik über Controller, Helper und Browser-JavaScript verteilt (Phase 1, T-02). Das soll vermieden werden.

## Optionen
- **A: Modularer Monolith, fachlich geschnitten** – ein Prozess, ein Deployment, Module mit öffentlichen Service-Interfaces.
- B: Microservices – unabhängig skalierbar, aber mehr Betriebsaufwand und höherer Speicherbedarf, widerspricht dem Budget.
- C: Schichtenmonolith ohne Modulgrenzen – schnell am Anfang, erodiert aber erfahrungsgemäß.

## Entscheidung
Option A. Ein Go-Binary mit HTTP-API, Job-Worker und Scheduler. Die Module (Identity, Vehicles, Odometer, Fuel, Oil, Trips, Maintenance, ServiceHistory, Costs, Documents, Notes, Notifications, Import, optional Assistant) haben je:
- ein öffentliches Go-Interface (Application Service) – nur darüber sprechen andere Module mit ihm;
- eigene Tabellen – kein Modul liest oder schreibt Tabellen eines anderen;
- Domain-Events im Prozess (z. B. `odometer.reading_recorded`) für lose Kopplung.

HTTP-Handler, KI-Werkzeuge und Import rufen ausschließlich Application Services auf. Die Autorisierung (ADR-016) und die Plausibilitätsregeln (ADR-010) liegen in diesen Services, nie in Handlern oder Clients.

## Konsequenzen
- (+) Ein Deployment, niedriger Speicherbedarf, Module später extrahierbar.
- (+) Web, Android und Assistent nutzen dieselben Regeln (Auftrag 5.1).
- (−) Modulgrenzen müssen technisch durchgesetzt werden: Go-`internal`-Pakete und ein Import-Lint in der CI.

## Bezug
Auftrag 4, 5.1, 5.2; Phase 1 T-02, T-03; ADR-000.
