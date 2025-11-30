class OfflineQueueManager {
  constructor() {
    this.queue = this.loadQueue();
    this.isOnline = navigator.onLine;
    this.retryInProgress = false;
    
    window.addEventListener('online', () => this.handleOnline());
    window.addEventListener('offline', () => this.handleOffline());
    
    if (this.isOnline) {
      this.processQueue();
    }
  }

  loadQueue() {
    try {
      const saved = localStorage.getItem('offline_queue');
      return saved ? JSON.parse(saved) : [];
    } catch (e) {
      console.error('Error loading queue:', e);
      return [];
    }
  }

  saveQueue() {
    try {
      localStorage.setItem('offline_queue', JSON.stringify(this.queue));
    } catch (e) {
      console.error('Error saving queue:', e);
    }
  }

  addToQueue(item) {
    const queueItem = {
      id: Date.now() + Math.random(),
      timestamp: new Date().toISOString(),
      retryCount: 0,
      ...item
    };
    
    this.queue.push(queueItem);
    this.saveQueue();
    return queueItem.id;
  }

  removeFromQueue(id) {
    this.queue = this.queue.filter(item => item.id !== id);
    this.saveQueue();
  }

  updateQueueItem(id, updates) {
    const item = this.queue.find(item => item.id === id);
    if (item) {
      Object.assign(item, updates);
      this.saveQueue();
    }
  }

  handleOnline() {
    console.log('Connection restored');
    this.isOnline = true;
    this.processQueue();
  }

  handleOffline() {
    console.log('Connection lost');
    this.isOnline = false;
  }

  async processQueue() {
    if (this.retryInProgress || this.queue.length === 0 || !this.isOnline) {
      return;
    }

    this.retryInProgress = true;

    while (this.queue.length > 0 && this.isOnline) {
      const item = this.queue[0];
      
      try {
        await this.retryItem(item);
        this.removeFromQueue(item.id);
      } catch (error) {
        console.error('Retry failed:', error);
        item.retryCount++;
        
        if (item.retryCount >= 5) {
          this.removeFromQueue(item.id);
          this.notifyUser(item, 'failed');
        } else {
          this.saveQueue();
          break; 
        }
      }
    }

    this.retryInProgress = false;
  }

  async retryItem(item) {
    if (item.type === 'post') {
      return await this.retryPost(item);
    } else if (item.type === 'comment') {
      return await this.retryComment(item);
    }
  }

  async retryPost(item) {
    const formData = new FormData();
    formData.append('title', item.data.title);
    formData.append('content', item.data.content);
    item.data.categories.forEach(catID => formData.append('categories', catID));
    
    if (item.data.imageURL) {
      formData.append('image_url', item.data.imageURL);
    }

    const response = await fetch('/post/createpost', {
      method: 'POST',
      body: formData
    });

    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`);
    }

    this.notifyUser(item, 'success');
    return response;
  }

  async retryComment(item) {
    const formData = new URLSearchParams();
    formData.append('postid', item.data.postId);
    formData.append('comment', item.data.content);

    const response = await fetch('/post/addcommentREQ', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/x-www-form-urlencoded'
      },
      body: formData
    });

    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`);
    }

    const result = await response.json();
    this.notifyUser(item, 'success', result);
    return result;
  }

  notifyUser(item, status, data = null) {
    const event = new CustomEvent('queueItemProcessed', {
      detail: { item, status, data }
    });
    window.dispatchEvent(event);
  }

  getQueueCount() {
    return this.queue.length;
  }

  getQueueItems() {
    return [...this.queue];
  }
}

export default new OfflineQueueManager();