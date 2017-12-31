package table

import "github.com/bobonovski/gotm/matrix"

var (
	// [w, t]-th element counts how many times
	// word w has been assigned to topic t
	WordTopic matrix.Matrix
	// [d, t]-th element counts how many words
	// in d has been assigned to topic t
	DocTopic matrix.Matrix
	// vector of length topicNum: [t]-th element counts
	// how many words in total has been assigned to topic t
	WordTopicSum matrix.Matrix
	// hashmap which remembers the topic of i-th word of doc d
	// has been assigned before
	DocWordTopic map[DocWord]uint32
)

type DocWord struct {
	DocId   uint32
	WordIdx uint32
}

// initialize word-topic and doc-topic sufficient statistics tables
func Init(topicNum uint32, vocabSize uint32, docNum uint32) error {
	WordTopic = matrix.NewDenseMatrix(vocabSize, topicNum)
	DocTopic = matrix.NewDenseMatrix(docNum, topicNum)
	WordTopicSum = matrix.NewDenseMatrix(topicNum, uint32(1))
	DocWordTopic = make(map[DocWord]uint32)

	return nil
}
