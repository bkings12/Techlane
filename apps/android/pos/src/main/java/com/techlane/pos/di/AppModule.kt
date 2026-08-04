package com.techlane.pos.di

import android.content.Context
import androidx.room.Room
import com.jakewharton.retrofit2.converter.kotlinx.serialization.asConverterFactory
import com.techlane.pos.BuildConfig
import com.techlane.pos.data.local.CatalogDao
import com.techlane.pos.data.local.ChargeDao
import com.techlane.pos.data.local.JobDao
import com.techlane.pos.data.local.PosDatabase
import com.techlane.pos.data.local.SaleCacheDao
import com.techlane.pos.data.local.ServiceDao
import com.techlane.pos.data.remote.AuthHeaderInterceptor
import com.techlane.pos.data.remote.TechLaneApi
import com.techlane.pos.data.remote.TokenRefreshInterceptor
import dagger.Module
import dagger.Provides
import dagger.hilt.InstallIn
import dagger.hilt.android.qualifiers.ApplicationContext
import dagger.hilt.components.SingletonComponent
import kotlinx.serialization.json.Json
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.logging.HttpLoggingInterceptor
import retrofit2.Retrofit
import java.util.concurrent.TimeUnit
import javax.inject.Named
import javax.inject.Singleton

@Module
@InstallIn(SingletonComponent::class)
object AppModule {

    @Provides
    @Singleton
    @Named("apiBase")
    fun apiBase(): String = BuildConfig.API_BASE.let { if (it.endsWith("/")) it else "$it/" }

    @Provides
    @Singleton
    fun json(): Json = Json {
        ignoreUnknownKeys = true
        coerceInputValues = true
        explicitNulls = false
        isLenient = true
        // Without this, any request field that happens to equal its Kotlin default
        // is dropped from the body — and the Go side then reads its own zero value.
        // That is how `quantity = 1` became `quantity: 0` and every charge came
        // back "invalid quantity". Nulls are still omitted, via explicitNulls.
        encodeDefaults = true
    }

    @Provides
    @Singleton
    fun okHttp(
        auth: AuthHeaderInterceptor,
        refresh: TokenRefreshInterceptor,
    ): OkHttpClient = OkHttpClient.Builder()
        .addInterceptor(auth)
        .addInterceptor(refresh)
        .apply {
            if (BuildConfig.DEBUG) {
                addInterceptor(
                    HttpLoggingInterceptor().apply { level = HttpLoggingInterceptor.Level.BASIC },
                )
            }
        }
        // Daraja round-trips are slow; a short read timeout turns a live prompt
        // into a phantom failure the technician would re-charge for.
        .connectTimeout(20, TimeUnit.SECONDS)
        .readTimeout(45, TimeUnit.SECONDS)
        .writeTimeout(45, TimeUnit.SECONDS)
        .retryOnConnectionFailure(true)
        .build()

    @Provides
    @Singleton
    fun retrofit(
        client: OkHttpClient,
        json: Json,
        @Named("apiBase") base: String,
    ): Retrofit = Retrofit.Builder()
        .baseUrl(base)
        .client(client)
        .addConverterFactory(json.asConverterFactory("application/json".toMediaType()))
        .build()

    @Provides
    @Singleton
    fun api(retrofit: Retrofit): TechLaneApi = retrofit.create(TechLaneApi::class.java)

    @Provides
    @Singleton
    fun database(@ApplicationContext context: Context): PosDatabase =
        Room.databaseBuilder(context, PosDatabase::class.java, "techlane_pos.db")
            .fallbackToDestructiveMigration()
            .build()

    @Provides
    fun catalogDao(db: PosDatabase): CatalogDao = db.catalogDao()

    @Provides
    fun serviceDao(db: PosDatabase): ServiceDao = db.serviceDao()

    @Provides
    fun chargeDao(db: PosDatabase): ChargeDao = db.chargeDao()

    @Provides
    fun jobDao(db: PosDatabase): JobDao = db.jobDao()

    @Provides
    fun saleCacheDao(db: PosDatabase): SaleCacheDao = db.saleCacheDao()
}
