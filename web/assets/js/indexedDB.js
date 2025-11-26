let db;

export function initDB() {
    const request = indexedDB.open("forum", 1);

    request.onupgradeneeded = function (e) {
        db = e.target.result;
        if (!db.objectStoreNames.contains('queue')) { // table queue
            db.createObjectStore("queue", { keyPath: "id", autoIncrement: true });
        }
    };

    request.onsuccess = function (e) {
        db = e.target.result;
        console.log("Database initialized");
    };

    request.onerror = function (e) {
        console.error("IndexedDB error:", e.target.errorCode);
    };
} 

export function saveOfflineRequest(url, data) {
    if (!db) {
        console.error("DB is not initialized");
        return;
    }
    const tx = db.transaction("queue", "readwrite");
    const store = tx.objectStore("queue");

    tx.oncomplete = function () {
        console.log("Saved request to " + url + " when offline");
    };

    tx.onerror = function (event) {
        console.error("Transaction error:", event.target.error);
    };

    tx.onabort = function (event) {
        console.error("Transaction aborted:", event.target.error);
    };

    store.add({
        url,
        data,
        createdAt: Date.now()
    });
}

export function retryPendingRequests() {
    if (!db) {
        console.error("DB is not initialized");
        return;
    }
    const tx = db.transaction("queue", "readwrite");
    const store = tx.objectStore("queue");

    tx.onerror = function (event) {
        console.error("Transaction error on retry:", event.target.error);
    };

    const req = store.openCursor();

    req.onsuccess = function (e) {
        const cursor = e.target.result;
        if (cursor) {
            const { url, data, id } = cursor.value;

            const xhr = new XMLHttpRequest();
            xhr.open("POST", url, true);
            xhr.setRequestHeader("Content-Type", "application/x-www-form-urlencoded");

            xhr.onload = function () {
                if (xhr.status >= 200 && xhr.status < 300) {
                    console.log("Successfully sent request to ", url);
                    // Need a new transaction to delete
                    const deleteTx = db.transaction("queue", "readwrite");
                    deleteTx.objectStore("queue").delete(cursor.key);
                } else {
                    console.error("Failed to send request to", url, "with status", xhr.status);
                }
            };
            
            xhr.onerror = function() {
                console.log("Network error on retry for request to", url);
            };

            xhr.send(data);
            cursor.continue();
        } else {
        }
    };

    req.onerror = function(e) {
        console.error("Error opening cursor:", e.target.error);
    };
}

