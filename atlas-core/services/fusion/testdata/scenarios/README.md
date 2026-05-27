# Fusion eval scenarios

Offline fixtures under `testdata/scenarios/` exercise the fusion eval harness (`eval` package) and `atlas-fusion-eval`.

## Formats

| Format | File | Description |
|--------|------|-------------|
| Static | `scenario.json` | Fixed observation rows |
| Simulated | `simulation.json` | Target motion + sensor feeds; materialized at eval time |

Optional `engines` in JSON limits which engines run (intersected with CLI `-engines`). When omitted, all CLI engines run.

## Running

From `atlas-core`:

```bash
go test ./services/fusion/...
go run ./services/fusion/cmd/atlas-fusion-eval -engines reference
go run ./services/fusion/cmd/atlas-fusion-eval -engines multisensor
go run ./services/fusion/cmd/atlas-fusion-sim-gen -sim services/fusion/testdata/scenarios/moving_adsb_dual_cam
```

Production `atlas-fusion` selects engines via `ATLAS_FUSION_ENGINES` (comma-separated, e.g. `reference,multisensor`). When unset, `ATLAS_FUSION_ENABLE_REFERENCE_ENGINE` defaults to reference-only.

Recalibrate tolerances (after engine/sim changes):

```bash
FUSION_CALIBRATE=1 go test ./eval -run TestReportScenarioGroundTruthErrors -v
```

Tolerance is set to `max(10m, ceil(measured_error * 1.25))`.

## Scenario matrix

| # | Directory | Engines | Intent | Calibrated GT (m) |
|---|-----------|---------|--------|-------------------|
| 0 | `single_point` | reference | Static ADS-B point; protocol + counts | n/a |
| 1 | `moving_adsb_single_cam` | reference, multisensor | ADS-B + one camera (baseline dual-feed) | 10 |
| 2 | `moving_adsb_dual_cam` | multisensor | ADS-B + dual camera + identity | 17 |
| 3 | `moving_adsb_only` | reference, multisensor | ADS-B only | 10 |
| 4 | `stationary_adsb_only` | reference, multisensor | Zero-speed target | 10 |
| 5 | `moving_lob_only_dual_cam` | multisensor | Dual camera triangulation, no ADS-B | 56 |
| 6 | `moving_slow_dual_feed` | reference, multisensor | 5 m/s, 120 s | 10 |
| 7 | `moving_fast_dual_feed` | reference, multisensor | 100 m/s | 10 |
| 8 | `moving_adsb_high_noise` | reference, multisensor | 80 m position noise, 200 m uncertainty | 66 |
| 9 | `moving_adsb_heavy_drops` | reference, multisensor | 12% drop rate, 90 s | 10 |
| 10 | `moving_adsb_very_sparse` | reference, multisensor | 2 s ADS-B interval, 30 s | 10 |
| 11 | `moving_adsb_high_delay` | reference, multisensor | 200–2500 ms delivery delay | 10 |
| 12 | `moving_cam_high_bearing_noise` | multisensor | 3° bearing / 2° elevation noise | 288 |
| 13 | `moving_dual_cam_asymmetric_noise` | multisensor | South camera 4° bearing noise | 397 |
| 14 | `moving_adsb_inflated_uncertainty` | reference, multisensor | 500 m uncertainty radius | 10 |
| 15 | `moving_adsb_no_altitude` | multisensor | Omit `altitude_m` from ADS-B | 17 |
| 16 | `moving_adsb_no_uncertainty` | multisensor | Omit `uncertainty_radius_m` | 53 |
| 17 | `moving_adsb_full_identity` | multisensor | Full aircraft identity block | 24 |
| 18 | `moving_adsb_minimal_identity` | multisensor | `icao24` only | 14 |
| 19 | `moving_adsb_no_identity` | multisensor | No identity on ADS-B | 15 |
| 20 | `moving_dual_cam_wide_baseline` | multisensor | Cameras far apart | 11 |
| 21 | `moving_dual_cam_narrow_baseline` | multisensor | ~200 m baseline | 67 |
| 22 | `moving_lob_only_collinear` | multisensor | Co-located observers; expect no track | 0 |
| 23 | `moving_three_cameras` | multisensor | ADS-B + 3 cameras (averages all valid LOB pairs) | 20 |
| 24 | `moving_high_altitude` | multisensor | 10 km altitude | 11 |
| 25 | `moving_low_altitude` | multisensor | 150 m altitude | 11 |
| 26 | `stationary_lob_only_single_cam` | multisensor | Single LOB; expect no track | 0 |
| 27 | `moving_lob_only_single_cam` | multisensor | Single LOB while moving; no track | 0 |
| 28 | `moving_reference_ignores_lob` | reference | ADS-B + camera; reference uses point only | 10 |
| 29 | `moving_equator_adsb_dual_cam` | multisensor | Target near equator; local cameras | 10 |
| 30 | `moving_high_lat_adsb_dual_cam` | multisensor | 60°N target; local cameras | 10 |
| 31 | `moving_short_10s` | reference, multisensor | 10 s duration | 10 |
| 32 | `moving_long_300s` | reference, multisensor | 300 s duration | 35 |
| 33 | `moving_heading_north` | reference, multisensor | Heading 0° | 10 |
| 34 | `moving_heading_east` | reference, multisensor | Heading 90° | 10 |
| 35 | `moving_heading_south` | reference, multisensor | Heading 180° | 10 |
| 36 | `stationary_long_300s` | reference, multisensor | Stationary 300 s | 10 |

Negative scenarios (#22, #26, #27) set `track_updates: 0`, `provenance_records: 0`, and `ground_truth_tolerance_m: 0`.

## Default motion (NYC baseline)

Unless noted: start `2026-01-01T12:00:00Z`, 60 s, 40.7°N 74.0°W, 1200 m altitude, 55 m/s, heading 85°.
