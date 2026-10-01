Feature: Metrics Endpoint
  As the person running the pool,
  I want an anonymous metrics endpoint with runtime and HTTP metrics,
  so that I can see how the app behaves without logging in.

  Scenario: An anonymous client can scrape runtime metrics
    When an anonymous client requests "/metrics"
    Then the metrics response status is 200
    And the metrics response is in the Prometheus text format
    And the metrics output contains the series "go_goroutines"
    And the metrics output contains the series "go_memstats_alloc_bytes"
    And the metrics output contains the series "go_gc_duration_seconds"
    And the metrics output contains the series "process_cpu_seconds_total"
    And the metrics output contains the series "process_open_fds"

  Scenario: A served request is counted under its route pattern
    Given an anonymous client has requested "/login"
    When an anonymous client requests "/metrics"
    Then the metrics output contains a request count for route "GET /login" with status "200"
    And the metrics output contains a request duration for route "GET /login" with status "200"

  Scenario: An unknown path is counted without exposing the raw path
    Given an anonymous client has requested "/no/such/path/123"
    When an anonymous client requests "/metrics"
    Then the metrics output contains a request count for route "unmatched" with status "404"
    And the metrics output does not contain "/no/such/path/123"

  Scenario: A wrong method is counted as unmatched
    Given an anonymous client has sent a PUT to "/login"
    When an anonymous client requests "/metrics"
    Then the metrics output contains a request count for route "unmatched" with status "405"

  Scenario: A protected page without a session is counted under its route pattern
    Given an anonymous client has requested "/leaderboard"
    When an anonymous client requests "/metrics"
    Then the metrics output contains a request count for route "GET /leaderboard" with status "302"

  Scenario: The metrics output never contains a player email or login code
    Given the seeded player has requested a login code
    When an anonymous client requests "/metrics"
    Then the metrics output does not contain "basti@example.com"
    And the metrics output does not contain the issued login code
