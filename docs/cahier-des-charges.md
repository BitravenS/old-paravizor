# Cahier des charges - Paravizor

## 1. Objet

Ce cahier des charges definit le **but**, la **vision**, le **fonctionnement cible** et les **exigences** de Paravizor.

Paravizor est concu comme une plateforme de reconnaissance offensive pour le bug bounty et les chercheurs de cybersécurité, orientée automatisation, pilotage humain et extensibilité.

---

## 2. Vision du projet

Paravizor vise a devenir un copilote de recon complet qui permet de:

- transformer une recon manuelle en processus structure et reproductible,
- centraliser les donnees de decouverte, triage et priorisation,
- reduire le bruit operationnel pour se concentrer sur les cibles a valeur,
- combiner orchestration d'outils et assistance intelligente (IA),
- offrir une experience efficace en terminal, sans sacrifier la profondeur d'analyse.

La philosophie du produit repose sur trois principes:

1. **Contrôlabilité** : l'utilisateur garde la maîtrise des decisions.
2. **Traçabilité** : chaque resultat doit pouvoir être relie a son origine.
3. **Évolutivité** : pipelines, outils et strategies doivent pouvoir évoluer rapidement.

---

## 3. Problèmes a résoudre

Paravizor doit resoudre les limites classiques des workflows recon:

- enchainement fragile de scripts,
- heterogeneite des formats d'outils,
- difficultes de reprise apres interruption,
- perte de contexte entre les etapes,
- surcharge cognitive lors du tri des resultats,
- manque de standardisation entre projets.

Le produit doit apporter une reponse claire a ces points via un orchestrateur unifie et un socle de donnees coherent.

---

## 4. Public cible

- Chercheurs bug bounty independants.
- Pentesters souhaitant industrialiser la phase de reconnaissance.
- Equipes AppSec voulant des pipelines partages et auditables.

---

## 5. Objectifs fonctionnels

## 5.1 Gestion de projet recon

Le systeme doit permettre de:

- creer, charger, reprendre et organiser des projets,
- definir un scope precis (in-scope / out-of-scope),
- conserver l'historique et le contexte d'investigation,
- parametrer les contraintes d'execution par projet.

## 5.2 Orchestration pipeline

Le systeme doit permettre de:

- decrire des pipelines modulaires en etapes/noeuds,
- connecter les noeuds par routage conditionnel ou parallele,
- executer des outils externes de maniere coherente,
- traiter differents types d'items (domaines, URLs, IP, ports, findings, fichiers),
- chaîner exploration, filtrage, enrichment et triage.

## 5.3 Fiabilite et reprise

Le systeme doit:

- suivre l'etat des traitements en continu,
- detecter les interruptions et permettre une reprise propre,
- eviter la corruption de donnees,
- garantir une execution robuste sur des volumes importants.

## 5.4 Exploitabilite des resultats

Le systeme doit fournir:

- une vue claire des actifs decouverts,
- une classification des signaux importants,
- des exports exploitables pour des workflows externes,
- des artefacts de restitution pour faciliter la phase suivante (validation, exploitation, reporting).

---

## 6. Fonctionnement cible (vue metier)

Le parcours utilisateur cible est le suivant:

1. **Initialisation**
   - l'utilisateur demarre un nouveau projet et renseigne son scope.

2. **Configuration de campagne**
   - selection/adaptation d'un pipeline recon,
   - verification des prerequis outils,
   - parametrage de l'execution.

3. **Execution orchestree**
   - le pipeline lance les etapes de decouverte,
   - les sorties sont normalisees, enrichies et reroutees,
   - les statuts d'avancement sont visibles en temps reel.

4. **Triage et priorisation**
   - les resultats utiles sont filtrés et classés,
   - les surfaces d'attaque pertinentes sont mises en avant.

5. **Assistance IA et decision**
   - l'IA propose analyses et priorites,
   - l'utilisateur valide, ajuste ou rejette les recommandations.

6. **Restitution**
   - export des artefacts,
   - production d'une synthese recon exploitable.

---

## 7. Experience utilisateur attendue

## 7.1 Interface principale

Paravizor doit proposer une interface terminal claire et rapide avec:

- un espace d'accueil projet,
- une vue d'execution orientee pilotage,
- des panneaux de details consultables rapidement,
- des raccourcis clavier coherents,
- une lisibilite élevée mếme sur sessions longues.

## 7.2 Mode commande

Le produit doit egalement offrir un mode CLI pour:

- les usages headless,
- l'integration scripts/CI,
- l'operation sans interface interactive.

---

## 8. Integration de l'IA (axe central)

## 8.1 Role de l'IA dans Paravizor

L'IA doit etre un **accelerateur de comprehension et de decision**, non un remplacant de l'expertise humaine.

Elle intervient comme couche d'assistance pour:

- resumer les observations,
- expliquer les technologies et anomalies,
- prioriser les pistes d'investigation,
- suggerer des actions suivantes pertinentes,
- aider a formaliser la restitution.

## 8.2 Capacites IA attendues

1. **Synthese contextuelle**
   - transformer un grand volume de donnees recon en vue actionnable.

2. **Priorisation intelligente**
   - identifier les assets/endpoints/signaux a plus forte valeur.

3. **Recommandation operationnelle**
   - proposer des suites logiques (outils, filtres, pivots, hypotheses).

4. **Assistance pipeline**
   - recommander des ajustements de workflow selon les resultats observes.

5. **Aide au reporting**
   - structurer la synthese et les prochaines actions de recherche.

## 8.3 Garde-fous IA

L'integration IA doit respecter des contraintes strictes:

- validation explicite des actions critiques,
- transparence sur les recommandations,
- possibilite de désactivation complète,
- parametrage du fournisseur et du mode de consentement,
- protection des donnees sensibles.

## 8.4 Critère clé de succès IA

L'IA est considerée utile si elle **améliore concrètement la vitesse et la qualite des decisions**, sans générer de dépendance aveugle ni de bruit supplementaire.

---

## 9. Exigences non fonctionnelles

## 9.1 Robustesse

- le systeme doit rester stable sur longues campagnes,
- les erreurs doivent etre explicites et actionnables,
- l'execution ne doit pas compromettre l'integrite des donnees.

## 9.2 Performance

- le systeme doit supporter un volume important d'items,
- l'orchestration doit privilegier debit et fluidite,
- la consommation de ressources doit rester maitrisable.

## 9.3 Maintenabilite

- architecture modulaire,
- evolution simple des pipelines/outils,
- documentation claire pour contribuer et etendre.

## 9.4 Portabilite

- fonctionnement local, autonome et dockerizable.
- compatibilite environnements terminal standards,
- deploiement simple pour utilisateurs individuels ou equipes.

---

## 10. Livrables attendus

1. Une application operable en mode terminal pour piloter une recon complete.
2. Un systeme de projet, pipeline et outillage configurable.
3. Une couche d'assistance IA integree et gouvernee.
4. Des mecanismes de restitution/export exploitables.
5. Une documentation utilisateur et technique coherente.

---

## 11. Criteres d'acceptation produit

Paravizor sera considere conforme a ce cahier des charges si:

1. Un utilisateur peut passer du scope initial a une campagne recon structuree sans bricolage externe majeur.
2. Le flux de traitement est lisible, pilotable et reprenable.
3. Les resultats sont suffisamment organises pour accelerer l'analyse offensive.
4. L'IA apporte une valeur mesurable en priorisation/synthese/recommandation.
5. L'utilisateur conserve en permanence le controle des decisions.

---

## 12. Feuille de route cible

1. Renforcer l'experience de pilotage de campagne de bout en bout.
2. Industrialiser la restitution (artefacts + synthese exploitable).
3. Monter progressivement en puissance sur l'assistance IA.
4. Structurer un ecosysteme de pipelines/outils partageables.
5. Positionner Paravizor comme reference recon orientee decision.
