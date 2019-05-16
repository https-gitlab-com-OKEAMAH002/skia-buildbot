package baseline

import (
	"bytes"
	"fmt"
	"math/rand"
	"sort"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/types"
)

const (
	TEST_1 = "test-01"
	TEST_2 = "test-02"
)

func TestMergeableBaseline(t *testing.T) {
	unittest.SmallTest(t)

	// Test a simple case with two tests and two digests.
	randDigests := types.DigestSlice{randomDigest(), randomDigest()}
	sort.Sort(randDigests)

	b := types.Expectations{
		TEST_1: {randDigests[0]: types.UNTRIAGED, randDigests[1]: types.POSITIVE},
		TEST_2: {randDigests[0]: types.UNTRIAGED, randDigests[1]: types.NEGATIVE},
	}
	var tmpBuf bytes.Buffer
	_, _ = fmt.Fprintf(&tmpBuf, "%s %s:%s %s:%s\n", TEST_1, randDigests[0], "u", randDigests[1], "p")
	_, _ = fmt.Fprintf(&tmpBuf, "%s %s:%s %s:%s\n", TEST_2, randDigests[0], "u", randDigests[1], "n")
	expected := tmpBuf.String()
	testWriteReadBaseline(t, b, &expected)

	// Make sure it works for empty expectations.
	empty := ""
	testWriteReadBaseline(t, types.Expectations{}, &empty)
}

func TestMergeableBaselineEdgeCases(t *testing.T) {
	unittest.SmallTest(t)

	// Write errors.
	b := types.Expectations{
		TEST_1: {"some_digest": types.UNTRIAGED},
	}
	var buf bytes.Buffer
	assert.Error(t, WriteMergeableBaseline(&buf, b))

	// Read error for invalid digest.
	testContent := "test-1 some_digest:u\n"
	_, err := ReadMergeableBaseline(bytes.NewBuffer([]byte(testContent)))
	assert.Error(t, err)

	// Read error for non-sorted test names
	testContent = "b-test-1 5eacdcf6a9efd4cda6f3b943f02f7dc8:u\n" +
		"c-test-1 5eacdcf6a9efd4cda6f3b943f02f7dc8:u\n" +
		"a-test-1 5eacdcf6a9efd4cda6f3b943f02f7dc8:u\n"
	_, err = ReadMergeableBaseline(bytes.NewBuffer([]byte(testContent)))
	assert.Error(t, err)

	// Read error for non-sorted digests
	testContent = "a-test-1 5eacdcf6a9efd4cda6f3b943f02f7dc8:u\n" +
		"b-test-1 5eacdcf6a9efd4cda6f3b943f02f7dc8:u \n" +
		"c-test-1 93e3ba6c6a3726d8cbd551278b4943fe:p    5eacdcf6a9efd4cda6f3b943f02f7dc8:u\n"
	_, err = ReadMergeableBaseline(bytes.NewBuffer([]byte(testContent)))
	assert.Error(t, err)

	// Read error for duplicate digests.
	testContent = "a-test-1 5eacdcf6a9efd4cda6f3b943f02f7dc8:u\n" +
		"b-test-1 5eacdcf6a9efd4cda6f3b943f02f7dc8:u \n" +
		"c-test-1 5eacdcf6a9efd4cda6f3b943f02f7dc8:p 5eacdcf6a9efd4cda6f3b943f02f7dc8:u 93e3ba6c6a3726d8cbd551278b4943fe:p \n"
	_, err = ReadMergeableBaseline(bytes.NewBuffer([]byte(testContent)))
	assert.Error(t, err)

	// This should be treated like an empty file and read without error.
	testContent = "\n\n# some comment\n"
	b, err = ReadMergeableBaseline(bytes.NewBuffer([]byte(testContent)))
	assert.NoError(t, err)
	assert.Equal(t, types.Expectations{}, b)
}

func testWriteReadBaseline(t *testing.T, b types.Expectations, expBuf *string) {
	var buf bytes.Buffer
	assert.NoError(t, WriteMergeableBaseline(&buf, b))

	if expBuf != nil {
		assert.Equal(t, *expBuf, buf.String())
	}

	foundBaseline, err := ReadMergeableBaseline(&buf)
	assert.NoError(t, err)
	assert.Equal(t, b, foundBaseline)
}

const (
	hexLetters = "0123456789abcdef"
	md5Length  = 32
)

func randomDigest() types.Digest {
	ret := make([]byte, md5Length)
	for i := 0; i < md5Length; i++ {
		ret[i] = hexLetters[rand.Intn(len(hexLetters))]
	}
	return types.Digest(ret)
}
