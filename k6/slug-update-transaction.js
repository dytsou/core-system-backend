import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate } from 'k6/metrics';

// Custom metrics
const slugUpdateSuccessRate = new Rate('slug_update_success');
const duplicateSlugRate = new Rate('duplicate_slugs');
const raceConditionRate = new Rate('slug_race_conditions');

// Test configuration
export const options = {
  setupTimeout: '120s', // Allow 2 minutes for setup
  stages: [
    { duration: '30s', target: 10 },   // Ramp up to 10 users
    { duration: '1m', target: 20 },    // Ramp up to 20 users
    { duration: '1m', target: 0 },     // Ramp down
  ],
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const USER_UUIDS = __ENV.USER_UUIDS ? __ENV.USER_UUIDS.split(',') : [];

// Helper: extract cookie values from k6 response cookies
function extractCookies(responseCookies) {
  const cookies = {};
  for (const name in responseCookies) {
    if (responseCookies[name] && responseCookies[name].length > 0) {
      cookies[name] = responseCookies[name][0].value;
    }
  }
  return cookies;
}

// Helper: login
function login(userUuid) {
  try {
    const res = http.post(
      `${BASE_URL}/api/auth/login/internal`,
      JSON.stringify({ uid: userUuid }),
      { headers: { 'Content-Type': 'application/json' }, timeout: '10s' }
    );

    if (res.status === 0) {
      // Connection failed (DNS, network, timeout, etc.)
      const errorMsg = res.error || 'Unknown connection error';
      if (errorMsg.includes('no such host') || errorMsg.includes('lookup')) {
        console.error(`DNS/Network error: Cannot resolve or reach ${BASE_URL}`);
      } else if (errorMsg.includes('timeout') || errorMsg.includes('deadline exceeded')) {
        console.error(`Request timeout: Server at ${BASE_URL} did not respond within 10s`);
      } else if (errorMsg.includes('connection reset') || errorMsg.includes('reset by peer')) {
        console.error(`Connection reset: Server at ${BASE_URL} closed the connection`);
      } else {
        console.error(`Connection failed: ${errorMsg}`);
      }
      return null;
    }

    if (res.status === 200) {
      const cookies = extractCookies(res.cookies);
      if (cookies['access_token']) {
        return { cookies };
      }
    }
    console.error(`Login failed: status=${res.status}, body=${res.body}`);
    return null;
  } catch (e) {
    console.error(`Login exception: ${e.message}`);
    return null;
  }
}

// Helper: create organization
function createOrg(cookies, name, slug) {
  try {
    const res = http.post(
      `${BASE_URL}/api/orgs`,
      JSON.stringify({ name, description: 'Test org for k6 slug update', slug, metadata: {} }),
      { headers: { 'Content-Type': 'application/json' }, cookies, timeout: '30s' }
    );

    if (res.status === 0) {
      const errorMsg = res.error || 'Unknown connection error';
      if (errorMsg.includes('timeout') || errorMsg.includes('deadline exceeded')) {
        console.error(`Create org timeout: Request to ${BASE_URL}/api/orgs timed out after 30s`);
      } else if (errorMsg.includes('connection reset') || errorMsg.includes('reset by peer')) {
        console.error(`Create org connection reset: Server closed the connection`);
      } else {
        console.error(`Create org connection failed: ${errorMsg}`);
      }
      return null;
    }

    if (res.status === 201) {
      return JSON.parse(res.body);
    }
    console.error(`Create org failed: status=${res.status}, body=${res.body}`);
    return null;
  } catch (e) {
    console.error(`Create org exception: ${e.message}`);
    return null;
  }
}

// Helper: get organization by slug
function getOrg(cookies, orgSlug) {
  const res = http.get(
    `${BASE_URL}/api/orgs/${orgSlug}`,
    { headers: { 'Content-Type': 'application/json' }, cookies }
  );

  if (res.status === 200) {
    try {
      return JSON.parse(res.body);
    } catch (e) {
      return null;
    }
  }
  return null;
}

// Helper: update organization slug
// PUT /api/orgs/{slug}
// Body: { "name": "...", "description": "...", "slug": "new-slug", "dbStrategy": "...", "metadata": {} }
function updateOrgSlug(cookies, currentSlug, newSlug, name) {
  const res = http.put(
    `${BASE_URL}/api/orgs/${currentSlug}`,
    JSON.stringify({
      name: name,
      description: 'Test org for k6 slug update',
      slug: newSlug,
      dbStrategy: '',
      metadata: {},
    }),
    { headers: { 'Content-Type': 'application/json' }, cookies }
  );

  return {
    status: res.status,
    body: res.body,
    newSlug: res.status === 200 ? JSON.parse(res.body).slug : null,
  };
}

// Helper: check if slug exists (try to get org with that slug)
function checkSlugExists(cookies, slug) {
  const res = http.get(
    `${BASE_URL}/api/orgs/${slug}`,
    { headers: { 'Content-Type': 'application/json' }, cookies }
  );
  return res.status === 200;
}

// Setup - create org
export function setup() {
  if (USER_UUIDS.length === 0) {
    throw new Error('USER_UUIDS is required');
  }

  console.log(`Setting up test data for slug update transaction test... (BASE_URL: ${BASE_URL})`);

  const auth = login(USER_UUIDS[0]);
  if (!auth) {
    const errorMsg = `Failed to login during setup. Cannot reach ${BASE_URL}. ` +
      `Please verify the server is running and accessible. ` +
      `If testing locally, try: BASE_URL=http://localhost:8080`;
    throw new Error(errorMsg);
  }

  const timestamp = Date.now();
  const orgSlug = `k6-slug-test-${timestamp}`;
  const orgName = `K6 Slug Test ${timestamp}`;

  const org = createOrg(auth.cookies, orgName, orgSlug);
  if (!org) {
    throw new Error('Failed to create organization');
  }

  console.log(`Setup complete! Created org: ${orgSlug}`);

  return {
    orgSlug,
    orgName,
    orgId: org.id,
    userId: USER_UUIDS[0],
  };
}

// Main test - concurrent slug updates
export default function (data) {
  const userUuid = USER_UUIDS[Math.floor(Math.random() * USER_UUIDS.length)];

  const auth = login(userUuid);
  if (!auth) {
    check(false, { 'login succeeded': () => false });
    return;
  }

  // First, try to get org by the original slug (it might have been renamed)
  // If that fails, we'll try to track the current slug
  let currentOrg = getOrg(auth.cookies, data.orgSlug);
  let currentSlug = data.orgSlug;

  // If org was renamed, try to find it by checking slug history or use original
  // For this test, we'll use the original slug - if it's been renamed, 
  // subsequent updates will fail (which tests that slug changes are atomic)
  if (!currentOrg) {
    // Org might have been renamed by another VU
    // We'll try with the original slug - if it fails, that's expected
    currentSlug = data.orgSlug;
  } else {
    currentSlug = currentOrg.slug;
  }

  // Generate unique slug for this iteration
  const timestamp = Date.now();
  const vuId = __VU || Math.floor(Math.random() * 10000);
  const iterationId = __ITER || Math.floor(Math.random() * 10000);
  const newSlug = `k6-slug-update-${timestamp}-vu${vuId}-iter${iterationId}`;

  // Attempt to update slug
  const result = updateOrgSlug(auth.cookies, currentSlug, newSlug, `${data.orgName} (Update d)`);

  if (result.status === 200) {
    slugUpdateSuccessRate.add(1);

    // Verify the new slug is active and old slug is not
    sleep(0.2);
    const newSlugActive = checkSlugExists(auth.cookies, newSlug);
    const oldSlugActive = checkSlugExists(auth.cookies, currentSlug);

    check(newSlugActive, {
      'new slug is active': () => newSlugActive === true,
    });

    check(oldSlugActive, {
      'old slug is not active': () => oldSlugActive === false,
    });

    if (oldSlugActive && newSlugActive) {
      duplicateSlugRate.add(1);
      raceConditionRate.add(1);
      console.error(`Race condition: Both ${currentSlug} and ${newSlug} are active!`);
    } else {
      duplicateSlugRate.add(0);
      raceConditionRate.add(0);
    }
  } else if (result.status === 404) {
    // Expected - org slug was already changed by another concurrent request
    slugUpdateSuccessRate.add(0);
    // This is expected behavior in concurrent scenarios
  } else if (result.status === 400 || result.status === 409) {
    // Expected - slug already exists or validation error
    slugUpdateSuccessRate.add(0);
  } else {
    slugUpdateSuccessRate.add(0);
    console.error(`Unexpected update status: ${result.status}, body: ${result.body}`);
  }

  // Final verification: check that no duplicate active slugs exist
  // This is a correctness check - only one slug should ever be active for an org
  sleep(0.3);
  const finalOrg = getOrg(auth.cookies, currentSlug) || getOrg(auth.cookies, newSlug);

  if (finalOrg) {
    const finalSlug = finalOrg.slug;
    const alternateActive = (finalSlug === newSlug && checkSlugExists(auth.cookies, currentSlug)) ||
      (finalSlug === currentSlug && checkSlugExists(auth.cookies, newSlug));

    check(alternateActive, {
      'no duplicate active slugs': () => alternateActive === false,
    });
  }

  sleep(1);
}

// Helper: delete organization
function deleteOrg(cookies, orgSlug) {
  const res = http.del(
    `${BASE_URL}/api/orgs/${orgSlug}`,
    null,
    { headers: { 'Content-Type': 'application/json' }, cookies }
  );
  return res.status === 200 || res.status === 204;
}

// Teardown function - runs once after all VUs finish
export function teardown(data) {
  if (!data) {
    console.log('No test data to clean up');
    return;
  }

  console.log('Cleaning up test data...');

  // Login with the same user used in setup
  const auth = login(data.userId);
  if (!auth) {
    console.error('Failed to login during teardown - skipping cleanup');
    return;
  }

  try {
    // Try to delete using original slug
    console.log(`Attempting to delete org with original slug: ${data.orgSlug}`);
    const deleted = deleteOrg(auth.cookies, data.orgSlug);

    if (!deleted) {
      // Org may have been renamed during concurrent slug update tests
      // This is expected behavior - the org has a unique slug and won't interfere with other tests
      console.warn(`Org may have been renamed (slug was: ${data.orgSlug}, Org ID: ${data.orgId})`);
      console.warn('Org will remain in database but with a unique slug. Can be cleaned up manually if needed.');
    } else {
      console.log('Cleanup complete!');
    }
  } catch (error) {
    console.error(`Error during cleanup: ${error.message}`);
  }
}

export function handleSummary(data) {
  return {
    'stdout': JSON.stringify(data, null, 2),
    'slug-update-transaction-results.json': JSON.stringify(data, null, 2),
  };
}

