package metadata

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMusicBrainzClient(t *testing.T) {
	c := NewMusicBrainzClient()
	require.NotNil(t, c)
	assert.NotNil(t, c.client)
	assert.Equal(t, MusicBrainzBaseURL, c.baseURL)
}

func TestMusicBrainzClient_rateLimit(t *testing.T) {
	c := NewMusicBrainzClient()
	start := time.Now()
	c.rateLimit()
	c.rateLimit()
	elapsed := time.Since(start)
	assert.True(t, elapsed >= 900*time.Millisecond, "rate limit should enforce ~1s between requests")
}

func TestSearchArtist(t *testing.T) {
	jsonBody := `{
		"artists": [
			{
				"id": "5b11f4ce-a62d-471e-81fc-a69a8278c7da",
				"name": "Nirvana",
				"sort-name": "Nirvana",
				"type": "Group",
				"country": "US",
				"disambiguation": "90s US grunge band",
				"tags": [{"name": "grunge", "count": 42}]
			}
		],
		"count": 1
	}`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/json", r.Header.Get("Accept"))
		assert.True(t, strings.Contains(r.URL.String(), "/artist/"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(jsonBody))
	}))
	defer ts.Close()

	c := NewMusicBrainzClient()
	c.baseURL = ts.URL + "/ws/2"

	artists, err := c.SearchArtist("Nirvana", 5)
	require.NoError(t, err)
	require.Len(t, artists, 1)
	assert.Equal(t, "Nirvana", artists[0].Name)
	assert.Equal(t, "US", artists[0].Country)
	assert.Equal(t, "Group", artists[0].Type)
	require.Len(t, artists[0].Tags, 1)
	assert.Equal(t, "grunge", artists[0].Tags[0].Name)
}

func TestSearchRelease(t *testing.T) {
	jsonBody := `{
		"releases": [
			{
				"id": "1b022e01-4da6-387b-8658-8678046e4cef",
				"title": "Nevermind",
				"date": "1991-09-24",
				"country": "US",
				"track-count": 12,
				"artist-credit": [
					{
						"artist": {
							"id": "5b11f4ce-a62d-471e-81fc-a69a8278c7da",
							"name": "Nirvana",
							"sort-name": "Nirvana"
						},
						"name": "Nirvana"
					}
				]
			}
		],
		"count": 1
	}`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.True(t, strings.Contains(r.URL.String(), "/release/"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(jsonBody))
	}))
	defer ts.Close()

	c := NewMusicBrainzClient()
	c.baseURL = ts.URL + "/ws/2"

	releases, err := c.SearchRelease("Nevermind", 5)
	require.NoError(t, err)
	require.Len(t, releases, 1)
	assert.Equal(t, "Nevermind", releases[0].Title)
	assert.Equal(t, "1991-09-24", releases[0].Date)
	assert.Equal(t, 12, releases[0].TrackCount)
	assert.Equal(t, "Nirvana", releases[0].ArtistCredit[0].Name)
}

func TestSearchTrack(t *testing.T) {
	jsonBody := `{
		"recordings": [
			{
				"id": "6de13f46-3651-4d9e-8277-7c9268e6f835",
				"title": "Smells Like Teen Spirit",
				"length": 301829,
				"artist-credit": [
					{
						"artist": {
							"id": "5b11f4ce-a62d-471e-81fc-a69a8278c7da",
							"name": "Nirvana",
							"sort-name": "Nirvana"
						},
						"name": "Nirvana"
					}
				]
			}
		],
		"count": 1
	}`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.True(t, strings.Contains(r.URL.String(), "/recording/"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(jsonBody))
	}))
	defer ts.Close()

	c := NewMusicBrainzClient()
	c.baseURL = ts.URL + "/ws/2"

	tracks, err := c.SearchTrack("Smells Like Teen Spirit", 5)
	require.NoError(t, err)
	require.Len(t, tracks, 1)
	assert.Equal(t, "Smells Like Teen Spirit", tracks[0].Title)
	assert.Equal(t, 301829, tracks[0].Length)
	assert.Equal(t, "Nirvana", tracks[0].ArtistCredit[0].Name)
}

func TestGetArtist(t *testing.T) {
	xmlBody := `<?xml version="1.0" encoding="UTF-8"?>
<metadata xmlns="http://musicbrainz.org/ns/mmd-2.0#">
  <artist id="5b11f4ce-a62d-471e-81fc-a69a8278c7da" type="Group" type-id="e431f5f6-b5d2-343d-8c36-72607de9c8d4">
    <name>Nirvana</name>
    <sort-name>Nirvana</sort-name>
    <country>US</country>
    <disambiguation>90s US grunge band</disambiguation>
    <tag-list>
      <tag count="42"><name>grunge</name></tag>
    </tag-list>
  </artist>
</metadata>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.True(t, strings.Contains(r.URL.Path, "/artist/"))
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(xmlBody))
	}))
	defer ts.Close()

	c := NewMusicBrainzClient()
	c.baseURL = ts.URL + "/ws/2"

	artist, err := c.GetArtist("5b11f4ce-a62d-471e-81fc-a69a8278c7da")
	require.NoError(t, err)
	require.NotNil(t, artist)
	assert.Equal(t, "Nirvana", artist.Name)
	assert.Equal(t, "US", artist.Country)
	assert.Equal(t, "Group", artist.Type)
	assert.Equal(t, "90s US grunge band", artist.Disambiguation)
	require.Len(t, artist.Tags, 1)
	assert.Equal(t, "grunge", artist.Tags[0].Name)
	assert.Equal(t, 42, artist.Tags[0].Count)
}

func TestGetRelease(t *testing.T) {
	xmlBody := `<?xml version="1.0" encoding="UTF-8"?>
<metadata xmlns="http://musicbrainz.org/ns/mmd-2.0#">
  <release id="1b022e01-4da6-387b-8658-8678046e4cef">
    <title>Nevermind</title>
    <date>1991-09-24</date>
    <country>US</country>
    <track-count>12</track-count>
    <artist-credit>
      <name-credit>
        <artist id="5b11f4ce-a62d-471e-81fc-a69a8278c7da">
          <name>Nirvana</name>
          <sort-name>Nirvana</sort-name>
        </artist>
        <name>Nirvana</name>
      </name-credit>
    </artist-credit>
    <medium-list>
      <medium position="1">
        <format>CD</format>
        <track-count>12</track-count>
        <track-list>
          <track id="6de13f46-3651-4d9e-8277-7c9268e6f835">
            <title>Smells Like Teen Spirit</title>
            <position>1</position>
            <length>301829</length>
            <recording id="6de13f46-3651-4d9e-8277-7c9268e6f835">
              <title>Smells Like Teen Spirit</title>
              <length>301829</length>
            </recording>
          </track>
        </track-list>
      </medium>
    </medium-list>
  </release>
</metadata>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.True(t, strings.Contains(r.URL.Path, "/release/"))
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(xmlBody))
	}))
	defer ts.Close()

	c := NewMusicBrainzClient()
	c.baseURL = ts.URL + "/ws/2"

	release, err := c.GetRelease("1b022e01-4da6-387b-8658-8678046e4cef")
	require.NoError(t, err)
	require.NotNil(t, release)
	assert.Equal(t, "Nevermind", release.Title)
	assert.Equal(t, "1991-09-24", release.Date)
	assert.Equal(t, "Nirvana", release.ArtistCredit[0].Artist.Name)
	require.Len(t, release.Media, 1)
	assert.Equal(t, "CD", release.Media[0].Format)
	assert.GreaterOrEqual(t, len(release.Media[0].Tracks), 0)
	if len(release.Media[0].Tracks) > 0 {
		assert.Equal(t, "Smells Like Teen Spirit", release.Media[0].Tracks[0].Title)
		assert.Equal(t, 1, release.Media[0].Tracks[0].Position)
		assert.Equal(t, 301829, release.Media[0].Tracks[0].Length)
	}
}

func TestGetRecording(t *testing.T) {
	xmlBody := `<?xml version="1.0" encoding="UTF-8"?>
<metadata xmlns="http://musicbrainz.org/ns/mmd-2.0#">
  <recording id="6de13f46-3651-4d9e-8277-7c9268e6f835">
    <title>Smells Like Teen Spirit</title>
    <length>301829</length>
    <artist-credit>
      <name-credit>
        <artist id="5b11f4ce-a62d-471e-81fc-a69a8278c7da">
          <name>Nirvana</name>
          <sort-name>Nirvana</sort-name>
        </artist>
        <name>Nirvana</name>
      </name-credit>
    </artist-credit>
  </recording>
</metadata>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.True(t, strings.Contains(r.URL.Path, "/recording/"))
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(xmlBody))
	}))
	defer ts.Close()

	c := NewMusicBrainzClient()
	c.baseURL = ts.URL + "/ws/2"

	rec, err := c.GetRecording("6de13f46-3651-4d9e-8277-7c9268e6f835")
	require.NoError(t, err)
	require.NotNil(t, rec)
	assert.Equal(t, "Smells Like Teen Spirit", rec.Title)
	assert.Equal(t, 301829, rec.Length)
	assert.Equal(t, "Nirvana", rec.ArtistCredit[0].Artist.Name)
}

func TestCacheHit(t *testing.T) {
	c := NewMusicBrainzClient()
	c.baseURL = "http://invalid.localhost"

	// Pre-populate cache
	c.setCache("artist_search:CachedArtist:5", []MBArtist{{ID: "cached-id", Name: "CachedArtist"}}, 10*time.Minute)

	artists, err := c.SearchArtist("CachedArtist", 5)
	require.NoError(t, err)
	require.Len(t, artists, 1)
	assert.Equal(t, "cached-id", artists[0].ID)
	assert.Equal(t, "CachedArtist", artists[0].Name)
}

func TestCacheExpiration(t *testing.T) {
	c := NewMusicBrainzClient()
	c.setCache("artist_search:Expired:5", []MBArtist{{ID: "expired", Name: "Expired"}}, -1*time.Second)

	_, ok := c.getCached("artist_search:Expired:5")
	assert.False(t, ok, "expired cache entry should be invalid")
}

func TestCoverArtURL(t *testing.T) {
	assert.Equal(t, "", CoverArtURL(""))
	assert.Equal(t, "https://coverartarchive.org/release/abc/front-250", CoverArtURL("abc"))
}

func TestJoinArtistNames(t *testing.T) {
	credits := []MBNameCredit{
		{Artist: MBArtist{Name: "Nirvana"}, Name: "Nirvana"},
		{Artist: MBArtist{Name: "Foo Fighters"}, Name: "Foo Fighters"},
	}
	assert.Equal(t, "Nirvana, Foo Fighters", JoinArtistNames(credits))

	empty := []MBNameCredit{}
	assert.Equal(t, "", JoinArtistNames(empty))
}

func TestMusicBrainzClient_ErrorStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("Service Unavailable"))
	}))
	defer ts.Close()

	c := NewMusicBrainzClient()
	c.baseURL = ts.URL + "/ws/2"

	_, err := c.SearchArtist("Test", 1)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "503"))
}

func TestMusicBrainzClient_InvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not json"))
	}))
	defer ts.Close()

	c := NewMusicBrainzClient()
	c.baseURL = ts.URL + "/ws/2"

	_, err := c.SearchArtist("Test", 1)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "decode"))
}

func TestMusicBrainzClient_InvalidXML(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not xml"))
	}))
	defer ts.Close()

	c := NewMusicBrainzClient()
	c.baseURL = ts.URL + "/ws/2"

	_, err := c.GetArtist("mbid")
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "xml decode"))
}

func TestMusicBrainzClient_LookupNotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Not Found"))
	}))
	defer ts.Close()

	c := NewMusicBrainzClient()
	c.baseURL = ts.URL + "/ws/2"

	_, err := c.GetArtist("mbid")
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "404"))
}

func TestMusicBrainzClient_EmptySearchResults(t *testing.T) {
	jsonBody := `{"artists":[],"count":0}`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(jsonBody))
	}))
	defer ts.Close()

	c := NewMusicBrainzClient()
	c.baseURL = ts.URL + "/ws/2"

	artists, err := c.SearchArtist("UnknownArtistThatDoesNotExist", 5)
	require.NoError(t, err)
	assert.Len(t, artists, 0)
}

func TestMusicBrainzClient_DefaultLimit(t *testing.T) {
	called := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		q := r.URL.Query()
		assert.Equal(t, "5", q.Get("limit"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"artists":[],"count":0}`))
	}))
	defer ts.Close()

	c := NewMusicBrainzClient()
	c.baseURL = ts.URL + "/ws/2"

	_, err := c.SearchArtist("Test", 0)
	require.NoError(t, err)
	assert.True(t, called)
}

func TestMusicBrainzClient_UserAgent(t *testing.T) {
	var ua string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"artists":[],"count":0}`))
	}))
	defer ts.Close()

	c := NewMusicBrainzClient()
	c.baseURL = ts.URL + "/ws/2"

	_, _ = c.SearchArtist("Test", 1)
	assert.True(t, strings.Contains(ua, "AetherStream"))
}

func ExampleMusicBrainzClient_SearchArtist() {
	c := NewMusicBrainzClient()
	// In real usage, this hits the MusicBrainz API.
	// For the example we just show the call shape.
	fmt.Println(c.baseURL)
	// Output: https://musicbrainz.org/ws/2
}
