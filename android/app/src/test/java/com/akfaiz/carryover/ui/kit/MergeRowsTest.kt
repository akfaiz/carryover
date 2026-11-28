package com.akfaiz.carryover.ui.kit

import org.junit.Assert.assertEquals
import org.junit.Test

/** Which rows an animated list draws: those it has, and the ones that just left, where they were. */
class MergeRowsTest {
    /** The rows as letters, one on its way out in brackets: "a (b) c". */
    private fun List<ShownRow<String>>.drawn() = joinToString(" ") { if (it.leaving) "(${it.item})" else it.item }

    private fun merge(shown: List<ShownRow<String>>, vararg items: String) = mergeRows(shown, items.toList()) { it }

    @Test
    fun theFirstRowsJustShow() {
        assertEquals("a b c", merge(emptyList(), "a", "b", "c").drawn())
    }

    @Test
    fun aRowRemovedStaysInItsPlaceWhileItLeaves() {
        val abc = merge(emptyList(), "a", "b", "c")
        assertEquals("a (b) c", merge(abc, "a", "c").drawn())
        assertEquals("(a) b c", merge(abc, "b", "c").drawn())
        assertEquals("a b (c)", merge(abc, "a", "b").drawn())
    }

    @Test
    fun rowsRemovedTogetherKeepTheirOrder() {
        val abcd = merge(emptyList(), "a", "b", "c", "d")
        assertEquals("a (b) (c) d", merge(abcd, "a", "d").drawn())
        assertEquals("(a) (b) (c) (d)", merge(abcd).drawn())
    }

    @Test
    fun aRowStillLeavingStaysWhereItIsWhenTheListChangesAgain() {
        val leaving = merge(merge(emptyList(), "a", "b", "c"), "a", "c")
        assertEquals("a (b) c", merge(leaving, "a", "c").drawn())
        assertEquals("a (b) (c)", merge(leaving, "a").drawn())
        assertEquals("a (b) c d", merge(leaving, "a", "c", "d").drawn())
    }

    @Test
    fun aRowThatComesBackStopsLeaving() {
        val leaving = merge(merge(emptyList(), "a", "b"), "a")
        assertEquals("a b", merge(leaving, "a", "b").drawn())
    }

    @Test
    fun aRowAddedTakesItsPlaceInTheList() {
        val ac = merge(emptyList(), "a", "c")
        assertEquals("a b c", merge(ac, "a", "b", "c").drawn())
    }

    @Test
    fun aRowThatChangedShowsWhatItIsNowAndOneLeavingWhatItWas() {
        val first = mergeRows(emptyList(), listOf(1 to "old", 2 to "two")) { it.first }
        val next = mergeRows(first, listOf(1 to "new")) { it.first }
        assertEquals(listOf(1 to "new", 2 to "two"), next.map { it.item })
        assertEquals(listOf(false, true), next.map { it.leaving })
    }
}
