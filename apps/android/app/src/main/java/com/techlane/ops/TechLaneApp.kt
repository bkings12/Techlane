package com.techlane.ops

import android.app.Application
import androidx.room.Room
import com.techlane.ops.data.AppDatabase
import com.techlane.ops.data.TokenStore
import com.techlane.ops.sync.SyncScheduler

class TechLaneApp : Application() {
    lateinit var database: AppDatabase
        private set
    lateinit var tokenStore: TokenStore
        private set

    override fun onCreate() {
        super.onCreate()
        instance = this
        database = Room.databaseBuilder(this, AppDatabase::class.java, "techlane.db")
            .fallbackToDestructiveMigration()
            .build()
        tokenStore = TokenStore(this)
        SyncScheduler.ensurePeriodic(this)
    }

    companion object {
        lateinit var instance: TechLaneApp
            private set
    }
}
