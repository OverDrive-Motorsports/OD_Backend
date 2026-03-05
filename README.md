<!--
##
## OverDrive 2026
## All Technical rights reserved
##
## README.md - Project overview and backend quick-start documentation.
##
-->

<div align="center">

# OverDrive – VR Experience for Motorsport

**A Meta Quest VR application that reinvents the motorsport viewing experience with total immersion, interactivity, and customization.**

</div>

<br>

## Project Vision

Today, motorsport viewing remains mostly linear and passive, despite the wealth of available data. OverDrive transforms the spectator into an active participant by integrating telemetry and race information directly into a virtual environment.

**Key Objectives:**
- Make motorsport **explorable, understandable, and alive**.
- Combine **immersion, comprehension, and customization**.
- Provide an innovative alternative to traditional broadcasts.

<br>

## Key Features

| Category               | Details                                                                                     |
|-------------------------|---------------------------------------------------------------------------------------------|
| **VR Experience**       | Dedicated environment, 3D mini-circuit, cars updated in near real-time.                    |
| **Real-Time Data**      | Standings, gaps, strategies, key events (stops, penalties, incidents).                      |
| **Customization**       | Panel movement and resizing, saving user configurations.                                     |
| **Mobile Application**  | Simplified/expert mode, notifications, parallel tracking with VR.                           |

<br>

## Target Personas

OverDrive is designed for three main user profiles:
- **The analytical fan**: In-depth data analysis.
- **The curious viewer**: Simple understanding of race dynamics.
- **The tech enthusiast**: Seeking immersion and innovation.

The application adapts information density and hierarchy according to the user profile.

<br>

## Technical Architecture

**Recommended Stack:**
- **VR**: Unity (native Meta Quest)
- **Backend**: Go or Node.js/TypeScript
- **Database**: PostgreSQL + Redis
- **Mobile**: Flutter
- **Communication**: WebSocket

**Features:**
- Scalable and modular.
- Real-time oriented.
- Compatible with progressive scaling.

<br>

## Contributors

Project developed by the **OverDrive Team – Epitech Paris (2026)**.

| Name            |
|-----------------|
| Anthony El Achkar |
| Clément-Alexis Fournier |
| Mariia Semenchenko |
| Batien Leroux |
| Corto Morrow |

<br>

## Installation & Usage

The project is currently under development. No public version is available at this time.

## Backend V1 (Go)

Current backend entrypoint:

```bash
go run ./cmd/api
```

### Available endpoints

- `GET /health`
- `GET /getrace` (legacy alias)
- `POST /sendrace` (legacy alias)
- `GET /api/v1/race/getrace`
- `POST /api/v1/race/sendrace`
- `GET /api/v1/race/cache`
- `GET /api/v1/race/championship/drivers`
- `GET /api/v1/race/championship/constructors`
- `GET /api/v1/race/weather`
- `GET /api/v1/race/facts`
- `GET /api/v1/race/driver?driver_number=63`
- `GET /api/v1/race/standings/race`
- `GET /api/v1/race/video-url`

Example flow:

```bash
# 1) fetch from OpenF1 and cache in backend memory
curl "http://localhost:8080/api/v1/race/getrace?year=2025&country=Australia&meeting=Australian%20Grand%20Prix&driver_number=63"

# 2) send cached payload
curl -X POST "http://localhost:8080/api/v1/race/sendrace"
```

<br>

## License

To be determined.
