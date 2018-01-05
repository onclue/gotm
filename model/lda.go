package model

import (
	"log"
	"math"
	"math/rand"
	"time"

	"github.com/bobonovski/gotm/corpus"
	"github.com/bobonovski/gotm/matrix"
	"github.com/bobonovski/gotm/sstable"
	"github.com/bobonovski/gotm/util"
)

type lda struct {
	data     *corpus.Corpus
	alpha    float32 // document topic mixture hyperparameter
	beta     float32 // topic word mixture hyperparameter
	topicNum uint32
}

// NewLDA creates a lda instance with collapsed gibbs sampler
func NewLDA(dat *corpus.Corpus,
	topicNum uint32, alpha float32, beta float32) *lda {
	// init sufficient statistics sstable
	sstable.WordTopic = matrix.NewUint32Matrix(dat.VocabSize, topicNum)
	sstable.DocTopic = matrix.NewUint32Matrix(dat.DocNum, topicNum)
	sstable.WordTopicSum = matrix.NewUint32Matrix(topicNum, uint32(1))
	sstable.DocWordTopic = make(map[sstable.DocWord]uint32)

	return &lda{
		data:     dat,
		alpha:    alpha,
		beta:     beta,
		topicNum: topicNum,
	}
}

func (this *lda) Run(iter int) {
	// randomly assign topic to word
	rand.Seed(time.Now().Unix())
	dw := sstable.DocWord{}
	for doc, wcs := range this.data.Docs {
		for i, w := range corpus.ExpandWords(wcs) {
			// sample word topic
			k := uint32(rand.Int31n(int32(this.topicNum)))

			// update sufficient statistics
			sstable.WordTopic.Incr(w, k, uint32(1))
			sstable.DocTopic.Incr(doc, k, uint32(1))
			sstable.WordTopicSum.Incr(k, uint32(0), uint32(1))

			// update doc word topic assignment
			dw.DocId = doc
			dw.WordIdx = uint32(i)
			sstable.DocWordTopic[dw] = k
		}
	}

	for iterIdx := 0; iterIdx < iter; iterIdx += 1 {
		if iterIdx%10 == 0 {
			log.Printf("iter %5d, likelihood %f", iterIdx, this.Likelihood())
		}

		// collapsed gibbs sampling
		cumsum := make([]float32, this.topicNum)
		for doc, wcs := range this.data.Docs {
			for i, w := range corpus.ExpandWords(wcs) {
				// get the current topic of word w
				dw.DocId = doc
				dw.WordIdx = uint32(i)
				k := sstable.DocWordTopic[dw]

				// decrease corresponding sufficient statistics
				sstable.WordTopic.Decr(w, k, uint32(1))
				sstable.DocTopic.Decr(doc, k, uint32(1))
				sstable.WordTopicSum.Decr(k, uint32(0), uint32(1))

				// resample the topic
				for kidx := uint32(0); kidx < this.topicNum; kidx += 1 {
					docPart := this.alpha + float32(sstable.DocTopic.Get(doc, kidx))
					wordPart := (this.beta + float32(sstable.WordTopic.Get(w, kidx))) /
						(float32(sstable.WordTopicSum.Get(kidx, uint32(0))) +
							this.beta*float32(this.data.VocabSize))
					if kidx == 0 {
						cumsum[kidx] = docPart * wordPart
					} else {
						cumsum[kidx] = cumsum[kidx-1] + docPart*wordPart
					}
				}
				u := rand.Float32() * cumsum[this.topicNum-1]
				for kidx := uint32(0); kidx < this.topicNum; kidx += 1 {
					if u < cumsum[kidx] {
						k = kidx
						break
					}
				}

				// increase corresponding sufficient statistics
				sstable.WordTopic.Incr(w, k, uint32(1))
				sstable.DocTopic.Incr(doc, k, uint32(1))
				sstable.WordTopicSum.Incr(k, uint32(0), uint32(1))
				sstable.DocWordTopic[dw] = k
			}
		}
	}
}

// compute the posterior point estimation of word-topic mixture
// beta (Dirichlet prior) + data -> phi
func (this *lda) Phi() *matrix.Float32Matrix {
	phi := matrix.NewFloat32Matrix(this.data.VocabSize, this.topicNum)

	for k := uint32(0); k < this.topicNum; k += 1 {
		sum := util.VectorSum(sstable.WordTopic.GetCol(k))

		for v := uint32(0); v < this.data.VocabSize; v += 1 {
			result := (float32(sstable.WordTopic.Get(v, k)) + this.beta) /
				(float32(sum) + float32(this.data.VocabSize)*this.beta)
			phi.Set(v, k, result)
		}
	}

	return phi
}

// compute the posterior point estimation of document-topic mixture
// alpha (Dirichlet prior) + data -> theta
func (this *lda) Theta() *matrix.Float32Matrix {
	theta := matrix.NewFloat32Matrix(this.data.DocNum, this.topicNum)

	for d := uint32(0); d < this.data.DocNum; d += 1 {
		sum := util.VectorSum(sstable.DocTopic.GetRow(d))

		for k := uint32(0); k < this.topicNum; k += 1 {
			result := (float32(sstable.DocTopic.Get(d, k)) + this.alpha) /
				(float32(sum) + float32(this.topicNum)*this.alpha)
			theta.Set(d, k, result)
		}
	}

	return theta
}

// compute the joint likelihood of corpus
func (this *lda) Likelihood() float64 {
	phi := this.Phi()
	theta := this.Theta()

	sum := float64(0.0)
	for doc, wcs := range this.data.Docs {
		for _, w := range corpus.ExpandWords(wcs) {
			topicSum := float32(0.0)
			for k := uint32(0); k < this.topicNum; k += 1 {
				topicSum += phi.Get(w, k) * theta.Get(doc, k)
			}
			sum += math.Log(float64(topicSum))
		}
	}

	return sum
}
