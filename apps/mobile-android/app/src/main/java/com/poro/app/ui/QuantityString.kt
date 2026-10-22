package com.poro.app.ui

import androidx.annotation.PluralsRes
import androidx.compose.runtime.Composable
import androidx.compose.runtime.ReadOnlyComposable
import androidx.compose.ui.platform.LocalContext

/** Texte au pluriel dont le nombre est aussi l'argument (« dans 1 seconde », « dans 42 secondes »). */
@Composable
@ReadOnlyComposable
fun quantityString(@PluralsRes id: Int, count: Int): String =
    LocalContext.current.resources.getQuantityString(id, count, count)
