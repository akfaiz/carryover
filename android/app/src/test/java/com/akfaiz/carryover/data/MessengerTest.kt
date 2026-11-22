package com.akfaiz.carryover.data

import com.akfaiz.carryover.R
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.fail
import org.junit.Test

private object IdTexts : Texts {
    override fun string(id: Int, vararg args: Any) = "$id:" + args.joinToString("|")
    override fun plural(id: Int, count: Int, vararg args: Any) = "p$id#$count:" + args.joinToString("|")
}

/** The messages the user is told, and how a failure is worded. */
class MessengerTest {
    private val m = Messenger(IdTexts)

    private fun next(): String = runBlocking { m.messages.first() }

    @Test
    fun aMessageIsDeliveredOnce() {
        m.say("hello")
        assertEquals("hello", next())
    }

    @Test
    fun messagesComeInOrder() {
        m.say("a")
        m.say("b")
        assertEquals("a", next())
        assertEquals("b", next())
    }

    @Test
    fun textsComeFromTheCatalog() {
        assertEquals("${R.string.msg_saved}:", m.str(R.string.msg_saved))
        assertEquals("${R.string.msg_sync_pushed}:Zelda", m.str(R.string.msg_sync_pushed, "Zelda"))
        assertEquals("p${R.plurals.msg_syncall_done}#2:2", m.plural(R.plurals.msg_syncall_done, 2, 2))
    }

    @Test
    fun aFailureSaysWhatFailedAndWhy() {
        m.failed("saving", IllegalStateException("disk full"))
        assertEquals("${R.string.msg_error}:saving|disk full", next())
    }

    @Test
    fun aFailureWithNoMessageFallsBackToTheException() {
        m.failed("saving", IllegalStateException())
        assertEquals("${R.string.msg_error}:saving|java.lang.IllegalStateException", next())
    }

    @Test
    fun cancellationIsNotAFailure() {
        try {
            m.failed("saving", CancellationException("cancelled"))
            fail("a cancellation must be thrown on")
        } catch (e: CancellationException) {
            assertEquals("cancelled", e.message)
        }
    }

    @Test
    fun attemptReturnsTheValue() = runBlocking {
        assertEquals(42, m.attempt("x") { 42 })
    }

    @Test
    fun attemptSaysAFailureAndReturnsNull() = runBlocking {
        assertNull(m.attempt("x") { error("boom") })
        assertEquals("${R.string.msg_error}:x|boom", m.messages.first())
    }

    @Test
    fun attemptDoesNotSwallowCancellation() {
        try {
            runBlocking { m.attempt("x") { throw CancellationException("stop") } }
            fail("expected cancellation")
        } catch (e: CancellationException) {
            assertEquals("stop", e.message)
        }
    }
}
