import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

// Custom metrics
const tokenRefreshSuccessRate = new Rate('token_refresh_success');
const concurrentRefreshRate = new Rate('concurrent_refresh_issues');
const tokenRefreshDuration = new Trend('token_refresh_duration');
const loginSuccessRate = new Rate('login_success');

// Test configuration
export const options = {
  stages: [
    { duration: '30s', target: 20 },   // Ramp up to 20 users
    { duration: '1m', target: 50 },    // Ramp up to 50 users
    { duration: '2m', target: 100 },   // Ramp up to 100 users
    { duration: '1m', target: 0 },     // Ramp down
  ],
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const USER_UUIDS = __ENV.USER_UUIDS ? __ENV.USER_UUIDS.split(',') : [];

// Helper function to extract cookie values from k6 response cookies
// k6 returns cookies as: { 'name': [{ name, value, ... }] }
// We need to convert to: { 'name': 'value' }
function extractCookies(responseCookies) {
  const cookies = {};
  for (const name in responseCookies) {
    if (responseCookies[name] && responseCookies[name].length > 0) {
      cookies[name] = responseCookies[name][0].value;
    }
  }
  return cookies;
}

// Helper function to login and get refresh token
// POST /api/auth/login/internal
// Body: { "uid": "user-uuid" }
function login(userUuid) {
  const loginUrl = `${BASE_URL}/api/auth/login/internal`;
  const payload = JSON.stringify({ uid: userUuid });
  const params = {
    headers: {
      'Content-Type': 'application/json',
    },
  };

  const res = http.post(loginUrl, payload, params);

  if (res.status === 200) {
    const cookies = extractCookies(res.cookies);
    const accessToken = cookies['access_token'];
    const refreshToken = cookies['refresh_token'];

    if (!refreshToken) {
      console.error(`Login returned 200 but no refresh_token cookie. User: ${userUuid}`);
      loginSuccessRate.add(0);
      return null;
    }

    loginSuccessRate.add(1);
    return {
      accessToken: accessToken,
      refreshToken: refreshToken,
      cookies: cookies,
    };
  }

  // Log error details
  loginSuccessRate.add(0);
  if (res.status === 404) {
    console.error(`Login failed: User not found (404). UUID: ${userUuid}`);
  } else {
    console.error(`Login failed: status=${res.status}, body=${res.body}`);
  }
  return null;
}

// Helper function to refresh token
// POST /api/auth/refresh
// Uses refresh_token cookie
function refreshToken(refreshTokenValue) {
  const url = `${BASE_URL}/api/auth/refresh`;
  const startTime = Date.now();

  const params = {
    headers: {
      'Content-Type': 'application/json',
    },
    cookies: {
      'refresh_token': refreshTokenValue,
    },
  };

  const res = http.post(url, null, params);
  const duration = Date.now() - startTime;
  tokenRefreshDuration.add(duration);

  // Extract new cookies if refresh succeeded
  const newCookies = extractCookies(res.cookies);

  return {
    status: res.status,
    newRefreshToken: newCookies['refresh_token'] || null,
    newAccessToken: newCookies['access_token'] || null,
    duration: duration,
    body: res.body,
  };
}

export default function () {
  // Validate configuration
  if (USER_UUIDS.length === 0) {
    console.error('USER_UUIDS is required. Set it in .env or pass via --env');
    return;
  }

  // Select a random user UUID
  const userUuid = USER_UUIDS[Math.floor(Math.random() * USER_UUIDS.length)];

  // Login to get initial refresh token
  const auth = login(userUuid);

  if (!auth || !auth.refreshToken) {
    check(false, { 'login succeeded': () => false });
    return;
  }

  check(auth.refreshToken, {
    'refresh token received': (token) => token && token.length > 0,
  });

  // Simulate multiple concurrent refresh token requests (same user, different devices)
  const numConcurrentRefreshes = 5;
  const refreshResults = [];

  for (let i = 0; i < numConcurrentRefreshes; i++) {
    const result = refreshToken(auth.refreshToken);
    refreshResults.push(result);

    if (result.status === 204) {
      tokenRefreshSuccessRate.add(1);
    } else if (result.status === 401 || result.status === 403 || result.status === 404) {
      // Token was invalidated by another concurrent refresh - expected behavior
      tokenRefreshSuccessRate.add(0);
      concurrentRefreshRate.add(1);
    } else {
      tokenRefreshSuccessRate.add(0);
      console.error(`Unexpected refresh status: ${result.status}, body: ${result.body}`);
    }

    // Small delay between concurrent requests
    sleep(0.05);
  }

  // Verify that at least one refresh succeeded
  const successfulRefreshes = refreshResults.filter(r => r.status === 204).length;

  check(successfulRefreshes, {
    'at least one refresh succeeded': (count) => count >= 1,
  });

  // Test sequential refreshes (normal usage pattern)
  const firstSuccess = refreshResults.find(r => r.status === 204 && r.newRefreshToken);
  if (firstSuccess) {
    sleep(0.5);
    const secondRefresh = refreshToken(firstSuccess.newRefreshToken);

    check(secondRefresh.status, {
      'sequential refresh succeeded': (status) => status === 204,
    });
  }

  sleep(1);
}

export function handleSummary(data) {
  return {
    'stdout': JSON.stringify(data, null, 2),
    'jwt-refresh-token-results.json': JSON.stringify(data, null, 2),
  };
}
