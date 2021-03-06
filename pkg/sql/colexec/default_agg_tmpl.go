// Copyright 2020 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

// {{/*
// +build execgen_template
//
// This file is the execgen template for default_agg.eg.go. It's formatted
// in a special way, so it's both valid Go and a valid text/template input.
// This permits editing this file with editor support.
//
// */}}

package colexec

import (
	"context"
	"unsafe"

	"github.com/cockroachdb/cockroach/pkg/col/coldata"
	"github.com/cockroachdb/cockroach/pkg/sql/colconv"
	"github.com/cockroachdb/cockroach/pkg/sql/colexecbase/colexecerror"
	"github.com/cockroachdb/cockroach/pkg/sql/colmem"
	"github.com/cockroachdb/cockroach/pkg/sql/execinfrapb"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/types"
)

type default_AGGKINDAgg struct {
	// {{if eq "_AGGKIND" "Ordered"}}
	orderedAggregateFuncBase
	// {{else}}
	hashAggregateFuncBase
	// {{end}}
	allocator *colmem.Allocator
	fn        tree.AggregateFunc
	ctx       context.Context
	// inputArgsConverter is managed by the aggregator, and this function can
	// simply call GetDatumColumn.
	inputArgsConverter *colconv.VecToDatumConverter
	resultConverter    func(tree.Datum) interface{}
	vec                coldata.Vec
	scratch            struct {
		// Note that this scratch space is shared among all aggregate function
		// instances created by the same alloc object.
		otherArgs []tree.Datum
	}
}

var _ aggregateFunc = &default_AGGKINDAgg{}

const sizeOfDefault_AGGKINDAgg = int64(unsafe.Sizeof(default_AGGKINDAgg{}))

func (a *default_AGGKINDAgg) Init(groups []bool, vec coldata.Vec) {
	// {{if eq "_AGGKIND" "Ordered"}}
	a.orderedAggregateFuncBase.Init(groups, vec)
	// {{else}}
	a.hashAggregateFuncBase.Init(groups, vec)
	// {{end}}
	a.vec = vec
	a.Reset()
}

func (a *default_AGGKINDAgg) Reset() {
	// {{if eq "_AGGKIND" "Ordered"}}
	a.orderedAggregateFuncBase.Reset()
	// {{else}}
	a.hashAggregateFuncBase.Reset()
	// {{end}}
	a.fn.Reset(a.ctx)
}

func (a *default_AGGKINDAgg) Compute(
	vecs []coldata.Vec, inputIdxs []uint32, inputLen int, sel []int,
) {
	// Note that we only need to account for the memory of the output vector
	// and not for the intermediate results of aggregation since the aggregate
	// function itself does the latter.
	a.allocator.PerformOperation([]coldata.Vec{a.vec}, func() {
		// {{if eq "_AGGKIND" "Ordered"}}
		if sel != nil {
			for convertedTupleIdx, origTupleIdx := range sel[:inputLen] {
				_ADD_TUPLE(a, a.groups, a.nulls, convertedTupleIdx, origTupleIdx)
			}
		} else {
			for convertedTupleIdx, origTupleIdx := 0, 0; origTupleIdx < inputLen; {
				_ADD_TUPLE(a, a.groups, a.nulls, convertedTupleIdx, origTupleIdx)
				convertedTupleIdx++
				origTupleIdx++
			}
		}
		// {{else}}
		// We don't need to check whether sel is non-nil because the hash
		// aggregator always uses non-nil sel to specify the tuples to be
		// aggregated. Also, the hash aggregator converts the batch "sparsely",
		// so converted values are at the same positions as the original ones.
		var convertedTupleIdx int
		for _, origTupleIdx := range sel[:inputLen] {
			convertedTupleIdx = origTupleIdx
			_ADD_TUPLE(a, a.groups, a.nulls, convertedTupleIdx, origTupleIdx)
		}
		// {{end}}
	})
}

// {{/*
// _ADD_TUPLE aggregates the tuple that is at position 'origTupleIdx' in the
// original batch but has the converted tree.Datum values at position
// 'convertedTupleIdx'. These indices are the same when there is no selection
// vector but could be different if there is one.
func _ADD_TUPLE(
	a *default_AGGKINDAgg, groups []bool, nulls *coldata.Nulls, convertedTupleIdx, origTupleIdx int,
) { // */}}
	// {{define "addTuple" -}}

	// {{if eq "_AGGKIND" "Ordered"}}
	if a.groups[origTupleIdx] {
		res, err := a.fn.Result()
		if err != nil {
			colexecerror.ExpectedError(err)
		}
		if res == tree.DNull {
			a.nulls.SetNull(a.curIdx)
		} else {
			coldata.SetValueAt(a.vec, a.resultConverter(res), a.curIdx)
		}
		a.curIdx++
		a.fn.Reset(a.ctx)
	}
	// {{else}}
	// Go around unused warning.
	_ = origTupleIdx
	// {{end}}
	// Note that the only function that takes no arguments is COUNT_ROWS, and
	// it has an optimized implementation, so we don't need to check whether
	// len(inputIdxs) is at least 1.
	firstArg := a.inputArgsConverter.GetDatumColumn(int(inputIdxs[0]))[convertedTupleIdx]
	for j, colIdx := range inputIdxs[1:] {
		a.scratch.otherArgs[j] = a.inputArgsConverter.GetDatumColumn(int(colIdx))[convertedTupleIdx]
	}
	if err := a.fn.Add(a.ctx, firstArg, a.scratch.otherArgs...); err != nil {
		colexecerror.ExpectedError(err)
	}

	// {{end}}
	// {{/*
} // */}}

func (a *default_AGGKINDAgg) Flush(outputIdx int) {
	// {{if eq "_AGGKIND" "Ordered"}}
	// Go around "argument overwritten before first use" linter error.
	_ = outputIdx
	outputIdx = a.curIdx
	a.curIdx++
	// {{end}}
	a.allocator.PerformOperation([]coldata.Vec{a.vec}, func() {
		res, err := a.fn.Result()
		if err != nil {
			colexecerror.ExpectedError(err)
		}
		if res == tree.DNull {
			a.nulls.SetNull(outputIdx)
		} else {
			coldata.SetValueAt(a.vec, a.resultConverter(res), outputIdx)
		}
	})
}

func newDefault_AGGKINDAggAlloc(
	allocator *colmem.Allocator,
	constructor execinfrapb.AggregateConstructor,
	evalCtx *tree.EvalContext,
	inputArgsConverter *colconv.VecToDatumConverter,
	numArguments int,
	constArguments tree.Datums,
	outputType *types.T,
	allocSize int64,
) *default_AGGKINDAggAlloc {
	var otherArgsScratch []tree.Datum
	if numArguments > 1 {
		otherArgsScratch = make([]tree.Datum, numArguments-1)
	}
	return &default_AGGKINDAggAlloc{
		aggAllocBase: aggAllocBase{
			allocator: allocator,
			allocSize: allocSize,
		},
		constructor:        constructor,
		evalCtx:            evalCtx,
		inputArgsConverter: inputArgsConverter,
		resultConverter:    GetDatumToPhysicalFn(outputType),
		otherArgsScratch:   otherArgsScratch,
		arguments:          constArguments,
	}
}

type default_AGGKINDAggAlloc struct {
	aggAllocBase
	aggFuncs []default_AGGKINDAgg

	constructor execinfrapb.AggregateConstructor
	evalCtx     *tree.EvalContext
	// inputArgsConverter is a converter from coldata.Vecs to tree.Datums that
	// is shared among all aggregate functions and is managed by the aggregator
	// (meaning that the aggregator operator is responsible for calling
	// ConvertBatch method).
	inputArgsConverter *colconv.VecToDatumConverter
	resultConverter    func(tree.Datum) interface{}
	// otherArgsScratch is the scratch space for arguments other than first one
	// that is shared among all aggregate functions created by this alloc. Such
	// sharing is acceptable since the aggregators run in a single goroutine
	// and they process functions one at a time.
	otherArgsScratch []tree.Datum
	// arguments is the list of constant (non-aggregated) arguments to the
	// aggregate, for instance, the separator in string_agg.
	arguments tree.Datums
	// returnedFns stores the references to all aggregate functions that have
	// been returned by this alloc. Such tracking is necessary since
	// row-execution aggregate functions need to be closed (unlike optimized
	// vectorized equivalents), and the alloc object is a convenient way to do
	// so.
	// TODO(yuzefovich): it might make sense to introduce Close method into
	// colexec.aggregateFunc interface (which would be a noop for all optimized
	// functions) and move the responsibility of closing to the aggregators
	// because they already have references to all aggregate functions.
	returnedFns []*default_AGGKINDAgg
}

var _ aggregateFuncAlloc = &default_AGGKINDAggAlloc{}
var _ Closer = &default_AGGKINDAggAlloc{}

func (a *default_AGGKINDAggAlloc) newAggFunc() aggregateFunc {
	if len(a.aggFuncs) == 0 {
		a.allocator.AdjustMemoryUsage(sizeOfDefault_AGGKINDAgg * a.allocSize)
		a.aggFuncs = make([]default_AGGKINDAgg, a.allocSize)
	}
	f := &a.aggFuncs[0]
	*f = default_AGGKINDAgg{
		allocator:          a.allocator,
		fn:                 a.constructor(a.evalCtx, a.arguments),
		ctx:                a.evalCtx.Context,
		inputArgsConverter: a.inputArgsConverter,
		resultConverter:    a.resultConverter,
	}
	f.scratch.otherArgs = a.otherArgsScratch
	a.allocator.AdjustMemoryUsage(f.fn.Size())
	a.aggFuncs = a.aggFuncs[1:]
	a.returnedFns = append(a.returnedFns, f)
	return f
}

func (a *default_AGGKINDAggAlloc) Close(ctx context.Context) error {
	for _, fn := range a.returnedFns {
		fn.fn.Close(ctx)
	}
	a.returnedFns = nil
	return nil
}
